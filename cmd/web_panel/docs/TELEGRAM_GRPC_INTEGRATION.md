# Telegram gRPC Integration Guide

## Overview

The web_panel now sends SP (Surat Peringatan/Warning Letters) via Telegram instead of WhatsApp. This is done through a gRPC connection to the `service-platform-telegram` service running on port 50044.

## Architecture

```
web_panel (gRPC Client)
    ↓ telegram_helpers.go
    ↓ gRPC (localhost:50044)
    ↓
service-platform/cmd/telegram (gRPC Server)
    ↓ Telegram Bot API
    ↓
Telegram Users
```

## Configuration

### 1. Config Files

**Location:** `/home/user/server/service-platform/internal/config/`

Added to both `conf.dev.yaml` and `conf.prod.yaml`:

```yaml
TELEGRAM_SERVICE:
  GRPC_HOST: "localhost"
  GRPC_PORT: 50044
  CONNECTION_TIMEOUT: 5  # seconds
  REQUEST_TIMEOUT: 30    # seconds
```

### 2. Config Struct

**File:** `service-platform/internal/config/config.go`

```go
TelegramService struct {
    GRPCHost          string `yaml:"GRPC_HOST"`
    GRPCPort          int    `yaml:"GRPC_PORT"`
    ConnectionTimeout int    `yaml:"CONNECTION_TIMEOUT"` // seconds
    RequestTimeout    int    `yaml:"REQUEST_TIMEOUT"`    // seconds
} `yaml:"TELEGRAM_SERVICE"`
```

## Implementation

### 1. Initialize Connection in main.go

Add this to `service-platform/cmd/web_panel/main.go` after database initialization:

```go
import (
    "service-platform/cmd/web_panel/controllers"
    "service-platform/internal/config"
    // ... other imports
)

func main() {
    // ... existing initialization code ...

    // Initialize database
    gormdb.InitDB()

    // Initialize Telegram gRPC connection
    cfg := config.WebPanel.Get()
    if err := controllers.InitTelegramConnection(
        cfg.TelegramService.GRPCHost,
        cfg.TelegramService.GRPCPort,
    ); err != nil {
        logrus.Warnf("⚠️ Telegram gRPC connection failed: %v - Will log only", err)
        logrus.Warn("SP documents will be logged to database but not sent to Telegram")
    } else {
        logrus.Info("✅ Telegram gRPC connection established successfully")
    }

    // ... rest of your code ...
}
```

### 2. Graceful Shutdown

Add connection cleanup on shutdown:

```go
func main() {
    // ... initialization ...

    // Setup signal handling for graceful shutdown
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

    go func() {
        <-sigChan
        logrus.Info("Shutting down gracefully...")
        
        // Close Telegram gRPC connection
        if err := controllers.CloseTelegramConnection(); err != nil {
            logrus.WithError(err).Warn("Failed to close Telegram gRPC connection")
        }
        
        os.Exit(0)
    }()

    // Start server
    router.Run(fmt.Sprintf(":%s", cfg.App.Port))
}
```

## How It Works

### 1. SP Sending Flow

When `sendUsing = "telegram"` in sp_technician_controllers.go:

```go
// Line 4320
sendUsing := "telegram" // whatsapp | email | telegram

// When sending SP (lines 5378, 5413, 5452, 5725, 5992)
if sendUsing == "telegram" {
    chatID, _ := GetChatIDFromPhone(phoneNumber)
    err := SendSPDocumentViaTelegram(
        forProject, recipientType, recipientName, 
        chatID, message, spFilePath, spNumber, phoneNumber,
        // ... other parameters
    )
}
```

### 2. Telegram Helper Function

**File:** `controllers/telegram_helpers.go`

```go
func SendSPDocumentViaTelegram(...params) error {
    // 1. Prepare document URL
    documentURL := fmt.Sprintf("%s/uploads/sp/%s", baseURL, fileName)
    
    // 2. Calculate 2-day response deadline
    deadline := calculateWorkingDayDeadline(now, 2)
    
    // 3. Create database record
    telegramMsg := sptechnicianmodel.SPTelegramMessage{...}
    
    // 4. Send via gRPC if connected
    if telegramGRPCConn != nil {
        client := pb.NewTelegramServiceClient(telegramGRPCConn)
        resp, err := client.SendDocument(ctx, &pb.SendTelegramDocumentRequest{
            ChatId:    chatID,
            Document:  documentURL,
            Caption:   caption,
            ParseMode: "Markdown",
        })
        // Update telegramMsg with response
    }
    
    // 5. Save to database
    dbWeb.Create(&telegramMsg)
}
```

### 3. Database Tracking

All sends are logged to `sp_telegram_messages` table:

- **Before Send:** `sent_success = false`, `response_status = "pending"`
- **After Success:** `sent_success = true`, `telegram_message_id = <msgID>`
- **After Failure:** `sent_success = false`, `error_message = <error>`

## Testing

### 1. Check Service Status

```bash
# Check if telegram service is running
sudo systemctl status service-platform-telegram

# Check port is listening
sudo netstat -pantul | grep 50044
```

Expected output:
```
tcp6       0      0 :::50044                :::*                    LISTEN      12345/telegram
```

### 2. Test Connection from web_panel

```bash
cd /home/user/server/web_panel

# Run with debug logging
go run main.go

# Look for log messages:
# ✅ Connected to Telegram gRPC service at localhost:50044
# ✅ SP-1 sent successfully to John Doe (technician) via Telegram - MsgID: 12345
```

### 3. Check Database

```bash
mysql -u swi -p db_web_panel_rm

# Query sent messages
SELECT id, recipient_name, recipient_type, number_of_sp, 
       sent_success, telegram_message_id, sent_at 
FROM sp_telegram_messages 
ORDER BY created_at DESC 
LIMIT 10;
```

### 4. Manual gRPC Test

Using grpcurl:

```bash
# List available services
grpcurl -plaintext localhost:50044 list

# Send test document
grpcurl -plaintext \
  -d '{
    "chat_id": "123456789",
    "document": "https://example.com/test.pdf",
    "caption": "Test SP Document",
    "parse_mode": "Markdown"
  }' \
  localhost:50044 \
  proto.TelegramService/SendDocument
```

## Troubleshooting

### Connection Refused

**Error:** `failed to connect to Telegram gRPC at localhost:50044: connection refused`

**Solutions:**
1. Check service is running: `systemctl status service-platform-telegram`
2. Start if needed: `sudo systemctl start service-platform-telegram`
3. Check logs: `journalctl -u service-platform-telegram -f`

### Chat Not Found

**Error:** `chat not found`

**Cause:** User hasn't started the bot or chat_id is incorrect

**Solution:**
1. User must send `/start` to the bot first
2. Check `telegram_users` table for correct chat_id
3. Implement user registration flow (see Phase 4-5 in TELEGRAM_MIGRATION_GUIDE.md)

### Documents Not Sending

**Symptoms:** Records in database but `sent_success = false`

**Debug:**
1. Check `error_message` field in database
2. Verify document URL is accessible: `curl <documentURL>`
3. Check Telegram service logs
4. Verify bot has permission to send documents

### Wrong Chat ID

**Current:** Using phone number as placeholder

**Solution:** Implement proper chat_id lookup:

```go
func GetChatIDFromPhone(phoneNumber string) (string, error) {
    var user TelegramUser
    err := dbWeb.Where("phone_number = ? AND is_verified = true", phoneNumber).
        First(&user).Error
    if err != nil {
        return "", fmt.Errorf("user not registered")
    }
    return user.ChatID, nil
}
```

## Next Steps

1. **Implement User Registration** (Phase 4-5):
   - Create `telegram_users` table
   - Add `/start` and `/verify [phone]` commands to service-platform bot
   - Implement `GetChatIDFromPhone()` with actual DB lookup

2. **Response Tracking** (Phase 6):
   - Add message handler in service-platform bot
   - Update `has_response` and `response_status` when users reply
   - Track response time and deadline expiration

3. **Deadline Monitoring** (Phase 7):
   - Create cron job `CheckExpiredSPDeadlines()`
   - Mark expired messages
   - Send reminders to HRD/managers

4. **Stock Opname via Telegram** (Line 5097):
   - Currently shows TODO message
   - Implement Excel report sending via Telegram
   - Use similar pattern to SP sending

## References

- **Telegram Service Guide:** `/home/user/server/service-platform/docs/TELEGRAM_SERVICE_GUIDE.md`
- **Migration Summary:** `/home/user/server/service-platform/cmd/web_panel/docs/TELEGRAM_MIGRATION_COMPLETE.md`
- **Proto Definitions:** `/home/user/server/service-platform/proto/telegram.proto`
- **Service Platform Telegram:** `/home/user/server/service-platform/cmd/telegram/main.go`

## Summary

✅ **Configuration Added:** Both dev and prod configs have Telegram service settings  
✅ **gRPC Client Implemented:** telegram_helpers.go connects to service-platform  
✅ **Database Tracking:** All sends logged with status and response tracking  
✅ **Graceful Fallback:** System logs if gRPC unavailable instead of crashing  
✅ **Ready to Use:** Just need to add `InitTelegramConnection()` to main.go

**Status:** Implementation complete, ready for integration testing! 🚀
