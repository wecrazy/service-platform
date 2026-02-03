# Implementation Checklist

## Phase 1: Code Setup ✅ (COMPLETED)

- [x] Create `SPTelegramMessage` model
- [x] Create gRPC client helper
- [x] Create Telegram sender functions
- [x] Update `sp_technician_controllers.go` to use Telegram
- [x] Update `automigrate_db.go`
- [x] Change `sendUsing` to "telegram"
- [x] Remove all TODO comments
- [x] Create migration guide

## Phase 2: Dependencies & Proto (REQUIRED)

- [ ] Copy proto files from service-platform to web_panel
  ```bash
  cd /home/user/server/web_panel
  ln -s ../service-platform/proto ./proto
  ```

- [ ] Update go.mod dependencies
  ```bash
  go get google.golang.org/grpc@latest
  go get google.golang.org/grpc/credentials/insecure@latest
  go mod tidy
  ```

- [ ] Verify imports work
  ```bash
  go build ./...
  ```

## Phase 3: Enable gRPC in Sender (REQUIRED)

- [ ] Edit `controllers/telegram_sp_sender.go`
  - [ ] Line ~8: Add import `pb "service-platform/proto"`
  - [ ] Line ~66-77: Uncomment actual gRPC client code
  - [ ] Line ~79-81: Remove simulation code
  - [ ] Line ~83: Update to use `resp.Success`, `resp.MessageId`

## Phase 4: Initialize Client in main.go (REQUIRED)

- [ ] Add gRPC client initialization
  ```go
  import "service-platform/cmd/web_panel/internal/grpcclient"
  
  // After database init:
  if err := grpcclient.InitTelegramGRPCClient("localhost", 9092); err != nil {
      logrus.Fatal("Failed to init Telegram client:", err)
  }
  defer grpcclient.CloseTelegramGRPCClient()
  logrus.Info("✅ Telegram gRPC client ready")
  ```

## Phase 5: Chat ID Mapping (REQUIRED)

- [ ] Create `telegram_users` table
  ```sql
  -- See TELEGRAM_MIGRATION_GUIDE.md for SQL
  ```

- [ ] Create `model/telegram_user.go` model
  ```go
  // See TELEGRAM_MIGRATION_GUIDE.md for code
  ```

- [ ] Update `database/automigrate_db.go`
  ```go
  &model.TelegramUser{},
  ```

- [ ] Implement `ConvertPhoneNumberToChatID()` in `telegram_sp_sender.go`
  ```go
  // Replace placeholder with actual DB query
  // See TELEGRAM_MIGRATION_GUIDE.md Step 6
  ```

## Phase 6: Bot Registration (REQUIRED)

- [ ] In service-platform, update telegram bot handlers
  - [ ] Add `/start` command handler
  - [ ] Add `/verify [phone]` command handler
  - [ ] Save chat_id and phone to `telegram_users` table
  - [ ] Send confirmation message

## Phase 7: Response Tracking (RECOMMENDED)

- [ ] Update telegram bot `HandleUpdate` function
  - [ ] Check for pending SP messages
  - [ ] Record response text and timestamp
  - [ ] Update `has_response = true`
  - [ ] Send acknowledgment message

## Phase 8: Deadline Checker (RECOMMENDED)

- [ ] Create `CheckExpiredSPDeadlines()` function
- [ ] Add to scheduler (run every hour)
- [ ] Test expiry notifications

## Phase 9: Configuration (REQUIRED)

- [ ] Update `config.yaml`
  ```yaml
  telegram:
    grpc_host: "localhost"
    grpc_port: 9092
  ```

- [ ] Verify service-platform config
  ```yaml
  telegram:
    bot_token: "YOUR_TOKEN"
    grpc_port: 9092
  ```

## Phase 10: Testing (REQUIRED)

### Unit Tests
- [ ] Test gRPC client initialization
- [ ] Test phone to chat_id conversion
- [ ] Test SP sending (mocked)

### Integration Tests
- [ ] Start service-platform telegram service
  ```bash
  cd /home/user/server/service-platform
  go run cmd/telegram/main.go
  ```

- [ ] Register test user
  - [ ] Send `/start` to bot
  - [ ] Send `/verify 628XXXXXXXXX`
  - [ ] Verify in database

- [ ] Trigger SP generation
  ```bash
  cd /home/user/server/web_panel
  go run main.go
  ```

- [ ] Verify SP sent
  - [ ] Check logs for "✅ SP-X sent to..."
  - [ ] Check `sp_telegram_messages` table
  - [ ] Verify user received message in Telegram

- [ ] Test response
  - [ ] Reply to bot with acknowledgment
  - [ ] Verify `has_response = true` in database

- [ ] Test deadline expiry
  - [ ] Wait or manually update deadline
  - [ ] Run deadline checker
  - [ ] Verify `deadline_expired = true`

## Phase 11: Production Rollout

- [ ] Backup database
- [ ] Deploy to production
- [ ] Monitor logs for errors
- [ ] Verify first few SPs sent successfully
- [ ] Document any issues

## Phase 12: Post-Deployment

- [ ] Monitor for 1 week
- [ ] Collect user feedback
- [ ] Check response rates
- [ ] Review deadline expirations
- [ ] Optimize as needed

## Quick Command Reference

```bash
# Start Telegram service
cd /home/user/server/service-platform && go run cmd/telegram/main.go

# Start web_panel
cd /home/user/server/web_panel && go run main.go

# Check logs
tail -f /home/user/server/service-platform/log/telegram.log
tail -f /home/user/server/service-platform/cmd/web_panel/log/app.log

# Query database
mysql -u user -p database_name
> SELECT * FROM sp_telegram_messages ORDER BY sent_at DESC LIMIT 10;
> SELECT * FROM telegram_users WHERE is_verified = true;
```

## Rollback Plan (If Needed)

If issues occur, you can quickly rollback:

1. **Change sendUsing back**:
   ```go
   // In sp_technician_controllers.go line 4321
   sendUsing := "whatsapp"  // Revert to whatsapp
   ```

2. **Restart service**:
   ```bash
   systemctl restart web_panel
   ```

3. **The old WhatsApp functions are still in the code**, so it will work immediately.

## Notes

- ✅ All new files are non-breaking - they don't interfere with existing code
- ✅ Old WhatsApp functions remain available as fallback
- ✅ Can switch between whatsapp/telegram/email by changing one variable
- ✅ Database migrations are additive - no data loss
- ✅ Telegram messages are tracked separately from WhatsApp messages

## Current Status

**Phase 1**: ✅ COMPLETED  
**Phase 2-12**: ⏳ PENDING (Follow guide to complete)

See [MIGRATION_SUMMARY.md](MIGRATION_SUMMARY.md) for overview and [TELEGRAM_MIGRATION_GUIDE.md](TELEGRAM_MIGRATION_GUIDE.md) for detailed instructions.
