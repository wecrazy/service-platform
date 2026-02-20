# Twilio WhatsApp Service Tests

This directory contains unit and integration tests for the Twilio WhatsApp service.

## Test Files

### `client_test.go`
Tests for the Twilio client package (`internal/twilio/client.go`):
- **TestNewClient**: Tests client initialization with valid credentials
- **TestNewClientMissingCredentials**: Tests error handling when credentials are missing
- **TestSendMessage**: Tests sending a text message via Twilio API
- **TestSendMediaMessage**: Tests sending media (images, documents, etc.) via Twilio API

### `service_test.go`
Tests for the gRPC service implementation:
- **TestGRPCServiceConnection**: Tests gRPC server startup and client connection
- **TestSendMessageRPC**: Tests the SendMessage RPC endpoint
- **TestGetMessageStatusRPC**: Tests the GetMessageStatus RPC endpoint

## Requirements

Before running tests, ensure:

1. **Twilio Account Setup**:
   - Sign up for a Twilio account: https://www.twilio.com/console
   - Get your Account SID and Auth Token from Account Settings
   - Set up WhatsApp Sandbox: https://www.twilio.com/console/sms/whatsapp/sandbox

2. **Configuration**:
   Update `internal/config/service-platform.dev.yaml` with your Twilio credentials:
   ```yaml
   twilio:
     whatsapp:
       account_sid: "ACxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
       auth_token: "your_auth_token_here"
       whatsapp_number: "whatsapp:+1234567890"
       host: "localhost"
       grpc_port: 50061
   ```

## Running Tests

### Run all Twilio tests:
```bash
go test -v ./tests/twilio/...
```

### Run specific test:
```bash
go test -v ./tests/twilio/ -run TestNewClient
```

### Run with coverage:
```bash
go test -v -cover ./tests/twilio/...
```

### Run from Makefile:
```bash
make test  # Runs all tests including Twilio
```

## Test Behavior

### With Valid Credentials
- Tests will attempt actual Twilio API calls
- You will be charged for successful messages sent
- Keep test recipient numbers limited to avoid high costs

### Without Credentials
- Tests will skip with informational messages
- Tests verify the gRPC service structure only
- No real messages are sent

## Integration Testing

For manual integration testing:

1. **Start the service**:
   ```bash
   make run-twilio-whatsapp
   ```

2. **In another terminal, send test messages**:
   ```bash
   # Build a simple gRPC client or use grpcurl:
   grpcurl -plaintext \
     -d '{"to": "+1234567890", "body": "Hello from Twilio WhatsApp"}' \
     localhost:50061 proto.TwilioWhatsAppService/SendMessage
   ```

3. **Check service health**:
   ```bash
   curl http://localhost:9097/health
   ```

## Environment Variables

Tests automatically load configuration from:
- `internal/config/service-platform.dev.yaml` (development)
- Or specify via `internal/config/service-platform.prod.yaml` for production overrides

## Troubleshooting

### Tests skip with "Twilio credentials not configured"
- Check credentials in `internal/config/service-platform.dev.yaml`
- Verify AccountSID, AuthToken, and WhatsApp number are set

### Connection refused errors
- Ensure MongoDB is running: `make mongo-up`
- Verify no other service is using the gRPC port (50061) or metrics port (9097)

### Twilio API errors
- Check credentials are correct in Twilio Console
- Verify WhatsApp Sandbox is active
- Ensure recipient number is valid in sandbox list

## Notes

- Tests use port `127.0.0.1:0` (random port) to avoid conflicts
- Tests create temporary gRPC servers that are cleaned up automatically
- Real Twilio API calls may take 5-10 seconds
