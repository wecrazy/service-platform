# Twilio WhatsApp Integration Guide

This guide explains how to set up and use the Twilio WhatsApp integration in this Go project.

## Prerequisites

1. **Twilio Account**: Register for a free trial at https://www.twilio.com/console
2. **WhatsApp Business Number**: Get your sandbox number or production number from Twilio Console

## Setup Steps

### 1. Get Your Credentials

1. Go to [Twilio Console](https://console.twilio.com)
2. Copy your **Account SID** and **Auth Token** from the dashboard
3. Go to **Messaging → WhatsApp → Sandbox Settings** to get your WhatsApp number (format: `whatsapp:+xxxxxxxxxxxx`)

### 2. Configure in YAML

Edit `internal/config/service-platform.dev.yaml` (or your environment config file) and update the Twilio section:

```yaml
twilio:
  account_sid: "ACxxxxxxxxxxxxxxxxxxxxxxxxxx"          # Your Twilio Account SID
  auth_token: "your_auth_token_here"                   # Your Twilio Auth Token
  whatsapp_number: "whatsapp:+1234567890"              # Your Twilio WhatsApp number
  host: "localhost"                                     # gRPC service host
  grpc_port: 50061                                      # gRPC service port
```

Replace the placeholder values with your actual Twilio credentials.

### 3. Run the Service

```bash
go run ./cmd/twilio/whatsapp/main.go
```

Or build and run:

```bash
go build -o twilio-whatsapp ./cmd/twilio/whatsapp
./twilio-whatsapp
```

## Usage

### Client Code (Go)

```go
import "service-platform/internal/whatsapp"

// Initialize Twilio WhatsApp client (reads from YAML config automatically)
client, err := whatsapp.NewTwilioClient()
if err != nil {
    log.Fatal(err)
}
defer client.Close()

// Send a simple text message
sid, err := client.SendMessage("+1234567890", "Hello from Twilio!")
if err != nil {
    log.Fatal(err)
}

// Send a message with media (image, video, document, etc.)
sid, err = client.SendMediaMessage("+1234567890", "https://example.com/image.jpg", "Check this out!")
if err != nil {
    log.Fatal(err)
}
```

### gRPC Service

The Twilio WhatsApp service is exposed as a gRPC server. You can call it from any language that supports gRPC:

```go
import pb "service-platform/proto"

// Connect to gRPC service
conn, _ := grpc.Dial("localhost:50061", grpc.WithInsecure())
client := pb.NewWhatsAppServiceClient(conn)

// Send message
response, _ := client.SendMessage(context.Background(), &pb.SendMessageRequest{
    To: "+1234567890",
    Content: &pb.MessageContent{
        ContentType: &pb.MessageContent_Text{
            Text: "Hello via gRPC!",
        },
    },
})
```

## Project Structure

```
internal/whatsapp/
├── client.go              # Original gRPC client
├── twilio_client.go       # Twilio SDK implementation
└── ...

cmd/twilio/whatsapp/
└── main.go               # Twilio WhatsApp gRPC service main entry point
```

## Configuration Priority

The Twilio client reads configuration from the YAML config file based on the current environment:
1. Checks `config_mode` setting in `conf.yaml`
2. Falls back to `ENV` environment variable
3. Falls back to `GO_ENV` environment variable
4. Defaults to `dev` if not specified

Environmental config files:
- `internal/config/service-platform.dev.yaml` - Development settings
- `internal/config/service-platform.prod.yaml` - Production settings (if available)

## Key Features

✅ Send text messages  
✅ Send media messages (images, videos, documents)  
✅ gRPC service for integration  
✅ Prometheus metrics support  
✅ Health check endpoint  
✅ Graceful shutdown  
✅ Configuration managed via YAML  

## Monitoring

- **Metrics**: http://localhost:9090/metrics
- **Health Check**: http://localhost:9090/health
- **gRPC**: localhost:50061 (or configured port)

## Error Handling

All errors are logged using Logrus. Check the logs for troubleshooting:

```bash
# View logs
tail -f log/app.log
```

Common errors:
- **Missing YAML configuration**: Ensure twilio section exists in your config file
- **Invalid configuration values**: Verify account_sid, auth_token, and whatsapp_number are correct
- **Invalid phone number**: Ensure phone numbers are in correct format (e.g., +1234567890)
- **Sandbox limitations**: Free trial accounts have limited functionality in sandbox mode

## References

- [Twilio WhatsApp API Documentation](https://www.twilio.com/docs/whatsapp)
- [Twilio Go SDK GitHub](https://github.com/twilio/twilio-go)
- [Twilio Account Console](https://console.twilio.com)
