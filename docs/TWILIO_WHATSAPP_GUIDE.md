# Twilio WhatsApp Integration - Best Practices Guide

This guide covers important concepts and best practices for implementing WhatsApp messaging with Twilio based on their official documentation.

## Key Concepts from Twilio Documentation

### 1. Phone Number Format (E.164)

**All phone numbers MUST be in E.164 format:**
- Format: `+[country code][number]`
- Examples:
  - Indonesia: `+6285173207755`
  - USA: `+14155238886`
  - UK: `+442071234567`

**What NOT to do:**
- ❌ `6285173207755` (missing +)
- ❌ `+62-85-1732-07755` (with dashes)
- ❌ `(62) 851-732-07-755` (with formatting)
- ❌ `whatsapp:+6285173207755` (when using in `to` parameter - framework adds prefix)

Our implementation validates this format before sending.

### 2. Phone Number Prefixes in Twilio Requests

When working with WhatsApp in Twilio API:
- **Default SMS/Voice format**: `+6285173207755`
- **WhatsApp format**: `whatsapp:+6285173207755`

Our SDK automatically adds the `whatsapp:` prefix when sending messages.

### 3. WhatsApp Sandbox (Testing/Development)

**For development/testing, your number must be in the Sandbox:**

1. Go to [Twilio Console → WhatsApp → Sandbox Settings](https://console.twilio.com/us/account/sms/whatsapp/learn)
2. Click "Join Sandbox" or manage participants
3. Add your phone number
4. **Twilio will send you a WhatsApp message** with a code
5. Reply with the code to verify

**Why your messages may not be arriving:**
- ❌ Number not in sandbox list
- ❌ Wrong phone number format
- ❌ Sandbox not activated
- ❌ Verification code not confirmed

### 4. Customer Service Window (24-Hour Rule)

**You can send free-form messages in two scenarios:**

1. **Within 24 hours of receiving a message** from the user
   - Any message type is allowed
   - No template required
   - Scope: The specific conversation with that user

2. **Outside 24-hour window** (or for unsolicited messages)
   - ❌ Messages MUST use pre-approved templates
   - Templates require Meta approval
   - Only for notifications, not conversations

**Implementation consideration:**
```go
// You can send freely here (within 24hr window):
client.SendMessage(userNumber, "Thanks for reaching out!")

// For messages outside this window, use approved templates:
// (Requires additional implementation - not in scope for now)
```

### 5. Message Status Callbacks

**Track message delivery in real-time:**

You can set a Status Callback URL to receive updates when:
- Message is queued
- Message is sent
- Message is delivered
- Message failed
- Message read

**Configuration:**
- In Twilio Console → WhatsApp Settings
- Or per-message via `StatusCallback` parameter

**Status flow:**
```
queued → sent → delivered → read
           ↘
             failed (with error code)
```

Our implementation logs the status but doesn't yet implement callback handling.

### 6. TwiML (Twilio Markup Language)

**TwiML is used for responding to INCOMING messages**, not sending them.

**Use case:** When a user sends a message to your Twilio WhatsApp number:
1. Twilio makes a webhook request to your callback URL
2. You respond with TwiML XML
3. Twilio executes the instructions (send reply, forward, etc.)

**Example TwiML Response (for receiving messages):**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<Response>
    <Message>Thanks for your message! We'll reply soon.</Message>
</Response>
```

**In our gRPC implementation:**
- We don't respond with TwiML (we use protocol buffers instead)
- We would need to configure a webhook URL in Twilio Console
- When messages arrive, they POST to our webhook endpoint
- We can then call `SendMessage()` to reply

### 7. Response MIME Types

When implementing webhook handlers, Twilio understands:
- `application/xml` or `text/xml` → TwiML instructions
- `text/plain` → Plain text message response

### 8. Webhook Request Format

When Twilio receives an inbound message, it POSTs:
```
From: whatsapp:+6285173207755
To: whatsapp:+14155238886
Body: Hello!
MediaUrl0: https://... (if media attached)
MessageSid: SMxxxxxxxx
```

### 9. Opt-In Requirements

**WhatsApp requires explicit user consent:**
- Users must opt-in to receive notifications
- Sending unsolicited messages can result in:
  - Account suspension
  - Number blocking
  - Permanent ban from WhatsApp

**Best practices:**
- Always collect explicit opt-in
- Implement opt-out handling
- Monitor opt-out requests
- Respect user preferences

## Our Go Implementation

### What We Have Now

✅ **Implemented:**
- E.164 phone number validation
- Text message sending
- Media message sending (images, docs, audio)
- Client initialization with config
- Proper error logging
- gRPC service wrapper

### What We Could Add

⏳ **Future enhancements:**
- Status callback URL configuration
- Message status polling/tracking
- Webhook receiver for incoming messages
- Template message support
- Message type detection
- Retry logic for failed messages
- Rate limiting protection
- Conversation/session tracking

## Testing Checklist

Before sending real messages:

- [ ] WhatsApp Sandbox is activated
- [ ] Your number is in the participant list
- [ ] Verification code was confirmed
- [ ] Phone number is in E.164 format (`+6285173207755`)
- [ ] Twilio Account SID and Auth Token are correct
- [ ] `config.dev.yaml` has real credentials

## Troubleshooting

### "The 'To' number +6285173207755 is not a valid phone number"

**Cause:** One of:
1. Wrong E.164 format
2. Testing in sandbox but number not approved

**Fix:**
1. Verify format: `+` + country code + number
2. Add number to WhatsApp Sandbox
3. Confirm verification code

### Messages queued but never delivered

**Typical causes:**
- Recipient not in sandbox list
- Customer service window expired (need template)
- WhatsApp number not validated/activated

### "Twilio Authentication Failed"

**Fix:**
- Check Account SID and Auth Token
- Verify in Twilio Console (not credentials of your account)
- Ensure they match what's in `config.dev.yaml`

## Links

- [Twilio WhatsApp API Overview](https://www.twilio.com/docs/whatsapp/api)
- [Twilio TwiML Documentation](https://www.twilio.com/docs/messaging/twiml)
- [WhatsApp Sandbox Guide](https://www.twilio.com/docs/whatsapp/quickstart/go)
- [E.164 Format Standard](https://www.twilio.com/docs/glossary/what-e164)
- [Message Status Callbacks](https://www.twilio.com/docs/messaging/guides/track-outbound-message-status)
- [WhatsApp Business Account Setup](https://www.twilio.com/docs/whatsapp/tutorial/whatsapp-business-account)

## Quick Start

```bash
# 1. Setup Twilio credentials
# Edit: internal/config/config.dev.yaml

# 2. Run the service
make run-twilio-whatsapp

# 3. Send a test message via gRPC
grpcurl -plaintext -d '{"to": "+6285173207755", "body": "Hello"}' \
  localhost:50061 proto.TwilioWhatsAppService/SendMessage

# 4. Check Twilio Console logs for delivery status
# https://console.twilio.com/us/account/messaging/whatsapp/logs
```
