# WhatsApp to Telegram Migration Guide - SP Technician

## Overview

This document explains the migration from WhatsApp to Telegram for sending SP (Surat Peringatan / Warning Letters) to technicians, SPLs, SACs, and HRD in the web_panel application.

## What Changed

### 1. **Configuration**
- `sendUsing` variable changed from `"whatsapp"` to `"telegram"` (line 4321)
- Messages are now sent via gRPC to the `service-platform` Telegram service

### 2. **New Models Created**

#### `SPTelegramMessage` ([sp_telegram_message.go](model/sp_technician_model/sp_telegram_message.go))
Tracks all SP documents sent via Telegram with:
- Recipient information (type, name, chat_id, phone)
- SP details (number, file path, violation, letter number)
- Telegram message tracking (message_id, sent_at, success status)
- Response tracking (has_response, response_at, response_text)
- Deadline management (response_deadline, deadline_expired)

### 3. **New Functions Created**

#### gRPC Client ([internal/grpcclient/telegram_client.go](internal/grpcclient/telegram_client.go))
- `InitTelegramGRPCClient()` - Initializes connection to Telegram gRPC service
- `GetTelegramClient()` - Returns the global Telegram client
- `Reconnect()` - Reconnects if connection drops

#### Telegram Sender ([controllers/telegram_sp_sender.go](controllers/telegram_sp_sender.go))
- `SendSPViaTelegram()` - Main function to send SP via Telegram
- `SendSPDocumentTelegram()` - Wrapper function for easy SP document sending
- `ConvertPhoneNumberToChatID()` - Maps phone numbers to Telegram chat IDs
- `calculateResponseDeadline()` - Calculates response deadline (2 working days)

### 4. **Updated Files**

#### [controllers/sp_technician_controllers.go](controllers/sp_technician_controllers.go)
- Line 4321: Changed `sendUsing` from "whatsapp" to "telegram"
- Line 5053: Updated TODO for Stock Opname reports
- Lines 5160-5460: Replaced WhatsApp sending with Telegram for:
  - Technicians
  - SPL (Supervisor)
  - SAC (Area Coordinator)  
  - HRD

#### [database/automigrate_db.go](database/automigrate_db.go)
- Added `SPTelegramMessage` model to auto-migration

## Implementation Steps

### Step 1: Copy Proto Files

Copy the Telegram proto files from service-platform to web_panel:

```bash
# Proto files are already at the root level
# service-platform/proto/ contains:
# - telegram.pb.go
# - telegram_grpc.pb.go
# No copying needed - just import service-platform/proto in your code
```

### Step 2: Update go.mod Dependencies

Add required dependencies to `service-platform/cmd/web_panel/go.mod`:

```bash
cd /home/user/server/web_panel

# Add gRPC dependencies
go get google.golang.org/grpc@latest
go get google.golang.org/grpc/credentials/insecure@latest

# Update go.mod and download dependencies
go mod tidy
```

### Step 3: Update telegram_sp_sender.go

Uncomment the actual gRPC code in [telegram_sp_sender.go](controllers/telegram_sp_sender.go):

```go
// Add this import at the top
import pb "service-platform/proto"

// In SendSPViaTelegram function, replace the simulation with:
client := pb.NewTelegramServiceClient(telegramClient.GetConnection())

resp, err := client.SendDocument(ctx, &pb.SendTelegramDocumentRequest{
    ChatId:    req.ChatID,
    Document:  documentURL,
    Caption:   req.MessageText,
    ParseMode: "Markdown",
})

if err != nil {
    errorMessage = err.Error()
    sentSuccess = false
    logrus.WithError(err).Error("Failed to send document via Telegram gRPC")
} else {
    sentSuccess = resp.Success
    errorMessage = resp.Message
    telegramMessageID = resp.MessageId
}
```

### Step 4: Initialize Telegram Client

In your web_panel main.go or initialization file, add:

```go
import (
    "service-platform/internal/config"
    "service-platform/cmd/web_panel/internal/grpcclient"
)

func main() {
    // ... existing code ...

    // Initialize Telegram gRPC Client
    cfg := config.WebPanel.Get()
    if err := grpcclient.InitTelegramGRPCClient(
        "localhost",  // or cfg.Telegram.GRPCHost
        9092,         // or cfg.Telegram.GRPCPort
    ); err != nil {
        logrus.WithError(err).Fatal("Failed to initialize Telegram gRPC client")
    }
    defer grpcclient.CloseTelegramGRPCClient()

    logrus.Info("✅ Telegram gRPC client initialized")

    // ... rest of your code ...
}
```

### Step 5: Create Chat ID Mapping Table

Users must interact with the Telegram bot before you can send them messages. Create a mapping table:

```sql
CREATE TABLE telegram_users (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL,
    
    chat_id VARCHAR(100) NOT NULL UNIQUE,
    phone_number VARCHAR(50) UNIQUE,
    username VARCHAR(255),
    first_name VARCHAR(255),
    last_name VARCHAR(255),
    
    is_verified BOOLEAN DEFAULT FALSE,
    verified_at TIMESTAMP NULL,
    
    last_interaction TIMESTAMP NULL,
    
    INDEX idx_phone (phone_number),
    INDEX idx_chat_id (chat_id)
);
```

Create the model:

```go
// model/telegram_user.go
package model

import (
    "time"
    "gorm.io/gorm"
)

type TelegramUser struct {
    ID        uint   `gorm:"primaryKey"`
    CreatedAt time.Time
    UpdatedAt time.Time
    DeletedAt gorm.DeletedAt `gorm:"index"`
    
    ChatID          string     `gorm:"column:chat_id;uniqueIndex;size:100"`
    PhoneNumber     string     `gorm:"column:phone_number;uniqueIndex;size:50"`
    Username        string     `gorm:"column:username;size:255"`
    FirstName       string     `gorm:"column:first_name;size:255"`
    LastName        string     `gorm:"column:last_name;size:255"`
    IsVerified      bool       `gorm:"column:is_verified;default:false"`
    VerifiedAt      *time.Time `gorm:"column:verified_at"`
    LastInteraction *time.Time `gorm:"column:last_interaction"`
}

func (TelegramUser) TableName() string {
    return "telegram_users"
}
```

### Step 6: Implement Chat ID Mapping

Update `ConvertPhoneNumberToChatID` in [telegram_sp_sender.go](controllers/telegram_sp_sender.go):

```go
func ConvertPhoneNumberToChatID(phoneNumber string) (string, error) {
    dbWeb := gormdb.GetDB()
    if dbWeb == nil {
        return "", fmt.Errorf("database connection is nil")
    }

    var telegramUser model.TelegramUser
    if err := dbWeb.Where("phone_number = ? AND is_verified = true", phoneNumber).
        First(&telegramUser).Error; err != nil {
        return "", fmt.Errorf("chat ID not found for phone number %s: user must interact with bot first", phoneNumber)
    }
    
    return telegramUser.ChatID, nil
}
```

### Step 7: Implement Bot Registration Flow

In the service-platform Telegram bot ([service-platform/internal/telegram](../service-platform/internal/telegram)), add handlers:

```go
// Handle /start command
func (h *TelegramHelper) handleStartCommand(update tgbotapi.Update) {
    chatID := update.Message.Chat.ID
    
    msg := tgbotapi.NewMessage(chatID, 
        "Selamat datang! Untuk verifikasi akun, silakan kirim nomor telepon Anda dengan format:\n"+
        "/verify 628123456789")
    h.bot.Send(msg)
}

// Handle /verify command
func (h *TelegramHelper) handleVerifyCommand(update tgbotapi.Update, phoneNumber string) {
    chatID := update.Message.Chat.ID
    
    // Sanitize phone number
    sanitized, err := sanitizePhoneNumber(phoneNumber)
    if err != nil {
        h.bot.Send(tgbotapi.NewMessage(chatID, "Nomor telepon tidak valid"))
        return
    }
    
    // Save to database
    user := TelegramUser{
        ChatID:          fmt.Sprintf("%d", chatID),
        PhoneNumber:     sanitized,
        Username:        update.Message.From.UserName,
        FirstName:       update.Message.From.FirstName,
        LastName:        update.Message.From.LastName,
        IsVerified:      true,
        VerifiedAt:      &time.Now(),
        LastInteraction: &time.Now(),
    }
    
    if err := h.db.Create(&user).Error; err != nil {
        h.bot.Send(tgbotapi.NewMessage(chatID, "Gagal menyimpan data"))
        return
    }
    
    h.bot.Send(tgbotapi.NewMessage(chatID, "✅ Verifikasi berhasil!"))
}
```

### Step 8: Implement Response Tracking

In the service-platform Telegram bot, monitor incoming messages:

```go
func (h *TelegramHelper) HandleUpdate(update tgbotapi.Update) {
    if update.Message == nil {
        return
    }
    
    chatID := fmt.Sprintf("%d", update.Message.Chat.ID)
    
    // Check if this is a response to an SP message
    var spMsg SPTelegramMessage
    err := h.db.Where("chat_id = ? AND has_response = false AND response_status = 'pending'", chatID).
        Order("sent_at DESC").
        First(&spMsg).Error
    
    if err == nil {
        // Found pending SP message, record the response
        now := time.Now()
        spMsg.HasResponse = true
        spMsg.ResponseAt = &now
        spMsg.ResponseText = update.Message.Text
        spMsg.ResponseStatus = "acknowledged"
        
        h.db.Save(&spMsg)
        
        // Send acknowledgment
        msg := tgbotapi.NewMessage(update.Message.Chat.ID,
            "✅ Terima kasih atas tanggapan Anda terhadap SP yang diterbitkan.")
        h.bot.Send(msg)
        
        return
    }
    
    // Handle other commands/messages...
}
```

### Step 9: Add Deadline Checker (Cron Job)

Create a scheduled task to check expired deadlines:

```go
func CheckExpiredSPDeadlines() {
    dbWeb := gormdb.GetDB()
    now := time.Now()
    
    var expiredMessages []SPTelegramMessage
    err := dbWeb.Where("has_response = false AND response_deadline < ? AND deadline_expired = false", now).
        Find(&expiredMessages).Error
    
    if err != nil {
        logrus.WithError(err).Error("Failed to check expired SP deadlines")
        return
    }
    
    for _, msg := range expiredMessages {
        msg.DeadlineExpired = true
        msg.DeadlineExpiredCheck = &now
        
        if err := dbWeb.Save(&msg).Error; err != nil {
            logrus.WithError(err).Errorf("Failed to mark SP message %d as expired", msg.ID)
            continue
        }
        
        logrus.Warnf("⚠️ SP-%d to %s (%s) expired without response", 
            msg.NumberOfSP, msg.RecipientName, msg.RecipientType)
        
        // Optionally send notification to HRD or escalate
    }
    
    logrus.Infof("Checked %d expired SP messages", len(expiredMessages))
}
```

Add to your scheduler:

```go
scheduler.Every(1).Hour().Do(CheckExpiredSPDeadlines)
```

## Configuration

Add to your `config.yaml`:

```yaml
telegram:
  grpc_host: "localhost"
  grpc_port: 9092
  bot_token: "YOUR_BOT_TOKEN"
  
sp_technician:
  response_deadline_days: 2  # Working days for response
  enable_telegram: true
  telegram_document_base_url: "https://yourserver.com/uploads/sp"
```

## Testing

### 1. Test Telegram Bot Registration

```bash
# 1. Start service-platform telegram service
cd /home/user/server/service-platform
go run cmd/telegram/main.go

# 2. Open Telegram and find your bot
# 3. Send: /start
# 4. Send: /verify 628123456789
```

### 2. Test SP Sending

```bash
# Run the SP checking job
cd /home/user/server/web_panel
go run main.go

# Check logs for:
# ✅ SP-1 sent to SPL [Name] via Telegram
```

### 3. Verify Database

```sql
-- Check sent messages
SELECT * FROM sp_telegram_messages 
ORDER BY sent_at DESC LIMIT 10;

-- Check verified users
SELECT * FROM telegram_users 
WHERE is_verified = true;

-- Check pending responses
SELECT recipient_name, recipient_type, number_of_sp, sent_at, response_deadline
FROM sp_telegram_messages
WHERE has_response = false AND response_status = 'pending';
```

## Troubleshooting

### Issue 1: "chat ID not found for phone number"

**Cause**: User hasn't interacted with the bot yet.

**Solution**: 
1. User must send `/start` and `/verify [phone]` to the bot
2. Verify the phone number matches exactly (format: 628xxx)

### Issue 2: "telegram gRPC client is not initialized"

**Cause**: Client not initialized in main.go

**Solution**: Add `InitTelegramGRPCClient()` call in initialization

### Issue 3: "Failed to send document via Telegram"

**Cause**: Document URL not accessible or invalid

**Solution**:
1. Ensure SP PDF files are in web-accessible directory
2. Check `telegram_document_base_url` configuration
3. Verify file permissions

### Issue 4: SP messages not tracked

**Cause**: SPTelegramMessage model not migrated

**Solution**:
```bash
# Force migration
go run main.go migrate
```

## Migration Checklist

- [ ] Copy proto files to web_panel
- [ ] Update go.mod dependencies
- [ ] Uncomment gRPC code in telegram_sp_sender.go
- [ ] Initialize Telegram client in main.go
- [ ] Create telegram_users table
- [ ] Implement chat ID mapping
- [ ] Add bot registration handlers
- [ ] Add response tracking
- [ ] Add deadline checker cron job
- [ ] Update configuration files
- [ ] Test bot registration
- [ ] Test SP sending
- [ ] Verify database records

## Benefits of Telegram Migration

1. ✅ **Better Tracking**: All messages stored with message IDs
2. ✅ **Response Monitoring**: Track who responded and when
3. ✅ **Deadline Management**: Automatic expiry tracking
4. ✅ **No Phone Number Changes**: Telegram chat ID stays constant
5. ✅ **Rich Media Support**: Better document handling
6. ✅ **Read Receipts**: Know when messages are seen
7. ✅ **Group Support**: Can send to Telegram groups
8. ✅ **Inline Keyboards**: Interactive buttons for acknowledgment

## Next Steps

1. Complete the implementation following this guide
2. Test with a small group of users first
3. Monitor logs and database for issues
4. Roll out to all users
5. Consider adding:
   - Automated reminders for pending responses
   - Escalation to HRD for expired deadlines
   - Statistics dashboard for SP tracking
   - Bulk operations for multiple SPs

## Support

For issues or questions:
- Check service-platform logs: `/home/user/server/service-platform/log/`
- Check web_panel logs: `/home/user/server/service-platform/cmd/web_panel/log/`
- Review Telegram guide: [TELEGRAM_SERVICE_GUIDE.md](../../service-platform/docs/TELEGRAM_SERVICE_GUIDE.md)
