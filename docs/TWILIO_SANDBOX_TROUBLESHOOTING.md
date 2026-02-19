# Twilio WhatsApp Sandbox Troubleshooting Guide

## Problem: Messages Not Arriving

If you're not receiving WhatsApp messages from your Twilio service, you're likely using the **Free Trial WhatsApp Sandbox**. This guide explains why and how to fix it.

## 🎯 Quick Fix Checklist

- [ ] Number is in Twilio WhatsApp Sandbox participants list
- [ ] Verification code was replied to
- [ ] Using correct E.164 format: `+6285173207755`
- [ ] Waiting 5-30 seconds (sandbox is slow)
- [ ] Twilio service is running: `make run-twilio-whatsapp`
- [ ] Checked Twilio logs for errors

## 📊 Sandbox vs Production

| Feature | Sandbox (Free Trial) | Production |
|---------|-------------------|-----------|
| **Sender** | Twilio shared number | Your business number |
| **Recipients** | Pre-approved only | Anyone (with opt-in) |
| **Speed** | 5-30 seconds | < 1 second |
| **Rate limit** | Very limited | Higher limits |
| **Templates** | Limited | Full access |
| **Cost** | Free | Per message |
| **Use case** | Testing | Real users |

## 🔧 Step-by-Step Troubleshooting

### Issue 1: "Can't find my number in sandbox"

**Problem:** You can't see where to add your phone number

**Solution:**
1. Open [Twilio Console](https://console.twilio.com/)
2. Click **Messaging** → **WhatsApp**
3. Look for **"Sandbox Learning Center"** or **"Learn"** tab
4. Find **"Manage Sandbox Participants"** section
5. Click the **"+"** button
6. Enter your number: `+6285173207755`

**What happens next:**
- Twilio sends you a WhatsApp message
- You receive: `"Your Twilio Sandbox Participant Verification"`
- Reply with the CODE shown (usually something like: `join 123456`)

### Issue 2: Number added but still no messages

**Problem:** Number is in sandbox but messages don't arrive

**Causes and fixes:**

#### A. Verification Code Not Confirmed
- [ ] Did you receive Twilio's WhatsApp message?
- [ ] Did you reply with the verification code?
- [ ] Check your WhatsApp chat with "Twilio Sandbox"

**If you didn't receive the message:**
1. Make sure your number is added to sandbox (Step 1 above)
2. Wait 30 seconds
3. Check all chats - it might go to "Other" folder

#### B. Wrong Phone Number Format
- [ ] Using E.164 format? `+6285173207755`
- [ ] Not using: `6285173207755` (missing +)
- [ ] Not using: `+62-85-1732` (with dashes)

**Test the format:**
```bash
# Correct format:
go test -v -run TestSendMessage ./tests/twilio/

# You'll see in logs:
# "to": "+6285173207755"
```

#### C. Service Not Running
- [ ] Is the service running?
- [ ] Run: `make run-twilio-whatsapp`
- [ ] You should see: `✅ Twilio WhatsApp Service Started Successfully`

#### D. Sandbox Not Active
- [ ] Is WhatsApp Sandbox enabled?
- [ ] Go to: https://console.twilio.com/us/account/sms/whatsapp/learn
- [ ] Look for: Status = **"Active Sandbox"**

### Issue 3: Message SID returned but no delivery

**Problem:** Your code returns a Message SID (like `SMxxxxxx`) but phone doesn't receive

**This is the MOST COMMON issue** - here's why and what to do:

1. **Check Twilio Logs:**
   - Go to: https://console.twilio.com/us/account/logs
   - Search for your Message SID
   - Look for error codes like:
     - `21211`: Invalid phone number format
     - `21614`: Number not in sandbox
     - `21610`: Account suspended/locked
     - `30003`: Authentication failed

2. **Wait longer:**
   - Sandbox messages take 5-30 seconds
   - Not instant like production
   - Try waiting a full 30 seconds

3. **Verify number again:**
   ```bash
   # If you think you forgot, re-add and re-verify:
   # 1. Go to sandbox settings
   # 2. Remove the number
   # 3. Re-add it
   # 4. Reply to verification message
   ```

### Issue 4: "Authentication Failed" errors

**Problem:** Getting auth errors when sending

**Check credentials:**
```bash
grep -A 5 "whatsapp:" internal/config/config.dev.yaml

# Should show SANDBOX credentials (not production):
# account_sid: "ACxxxxx...sandbox"
# auth_token: "your_sandbox_token"
```

**Fix:**
1. Go to https://console.twilio.com/us/account/keys
2. Copy your:
   - **Account SID** (starts with `AC`)
   - **Auth Token** (long string)
3. Update `internal/config/config.dev.yaml`:
   ```yaml
   twilio:
     whatsapp:
       account_sid: "AC___YOUR_SID___"
       auth_token: "___YOUR_TOKEN___"
   ```

## 🧪 Testing with the Sandbox Test Script

We've created a helper script to diagnose issues:

```bash
# Run the sandbox test:
make test-twilio-sandbox

# Or manually:
bash scripts/test-twilio-whatsapp.sh
```

**What it checks:**
- ✅ Twilio credentials in config
- ✅ Service is running
- ✅ gRPC connection works
- ✅ Shows sandbox status
- ✅ Sends test message
- ✅ Shows next steps

## 📋 When to Check Twilio Logs

After sending a test message, always check:

**Twilio Console Logs:**
- URL: https://console.twilio.com/us/account/logs
- Filter by: WhatsApp, or search by Message SID
- Look for:
  - `Status: queued` → message is pending
  - `Status: sent` → sent to WhatsApp servers
  - `Status: delivered` → arrived on phone
  - `error_code: 21614` → number not in sandbox
  - `error_code: 30003` → auth failed

**Your app logs:**
```bash
# Run with verbose logging:
make run-twilio-whatsapp

# Look for:
# ✅ "Message sent successfully. SID: SMxxxxxx"
# ❌ "Failed to send WhatsApp message: ..."
```

## 🚀 Upgrading from Sandbox

When ready for production (not free trial):

1. **Create a Business WhatsApp Account:**
   - Self-service: https://www.twilio.com/console/sms/whatsapp/self-signup
   - Requires: Business name, phone number, use case

2. **Verify with Meta:**
   - Twilio guides you through verification
   - Takes 2-7 business days
   - Links to your Meta Business Manager

3. **Update Config:**
   ```yaml
   twilio:
     whatsapp:
       account_sid: "AC___PROD_SID___"      # Production Account SID
       auth_token: "___PROD_TOKEN___"       # Production Auth Token
       whatsapp_number: "whatsapp:+62..."   # Your business number (not sandbox)
       host: "localhost"
       grpc_port: 50061
   ```

4. **Deploy to Production**
   - No code changes needed
   - Just update config with production credentials
   - Messages will go to any valid number (no sandbox restrictions)

## 💬 Message Status Flow in Sandbox

```
You call SendMessage()
        ↓
API accepts request → Returns SID (MessageSID=SMxxxxxx)
        ↓
Message queued in Twilio
        ↓
⏱️  5-30 seconds passes...
        ↓
Twilio sends to WhatsApp servers
        ↓
If number is in sandbox → ✅ Delivered to your phone
If number NOT in sandbox → ❌ Bounces (error 21614)
```

## 🎯 Success Indicators

You'll know it's working when you see:

```
✅ In your app logs:
   "WhatsApp message sent successfully. SID: SMxxxxxx"

✅ In Twilio Console logs:
   Status: delivered
   Direction: outbound-api

✅ On your phone:
   New WhatsApp message from "Twilio Sandbox"
```

## Questions?

- Twilio Support: https://help.twilio.com/
- Twilio Docs: https://www.twilio.com/docs/whatsapp
- Sandbox Docs: https://www.twilio.com/docs/whatsapp/quickstart
- Stack Overflow: Tag `twilio` and `whatsapp`

## Common Error Codes

| Code | Meaning | Fix |
|------|---------|-----|
| 21211 | Invalid phone number format | Use E.164: `+6285173207755` |
| 21614 | Number not in sandbox | Add number to sandbox in Console |
| 30003 | Authentication failed | Check Account SID and Auth Token |
| 30004 | Permission denied | Check account has WhatsApp enabled |
| 63000 | Internal server error | Retry after 60 seconds |

---

**Still stuck?** Run this diagnostic:
```bash
make test-twilio-sandbox
```

It will walk you through everything step by step!
