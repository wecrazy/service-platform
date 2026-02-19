#!/bin/bash
# Twilio WhatsApp Sandbox Testing Script
# This helps diagnose why messages aren't being received

set -e

echo "======================================"
echo "🧪 Twilio WhatsApp Sandbox Test"
echo "======================================"
echo ""

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Determine config mode (same pattern as health-check-all.sh)
CONFIG_MODE=$(yq '.config_mode' "internal/config/conf.yaml" 2>/dev/null | tr -d '"' || echo "dev")
CONFIG_FILE="internal/config/service-platform.${CONFIG_MODE}.yaml"

# Check if credentials are loaded
echo "${YELLOW}1. Checking Twilio configuration...${NC}"
if [ ! -f "$CONFIG_FILE" ]; then
    echo -e "${RED}❌ ERROR: $CONFIG_FILE not found${NC}"
    exit 1
fi

# Use yq to load configuration (same pattern as health-check-all.sh)
ACCOUNT_SID=$(yq '.twilio.whatsapp.account_sid' "$CONFIG_FILE" 2>/dev/null | tr -d '"')
AUTH_TOKEN=$(yq '.twilio.whatsapp.auth_token' "$CONFIG_FILE" 2>/dev/null | tr -d '"')
WHATSAPP_NUMBER=$(yq '.twilio.whatsapp.whatsapp_number' "$CONFIG_FILE" 2>/dev/null | tr -d '"')
GRPC_PORT=$(yq '.twilio.whatsapp.grpc_port' "$CONFIG_FILE" 2>/dev/null | tr -d '"')
METRICS_PORT=$(yq '.metrics.twilio_whatsapp_port' "$CONFIG_FILE" 2>/dev/null | tr -d '"' || echo "9097")

# Validate that values are not null/empty
if [ -z "$ACCOUNT_SID" ] || [ "$ACCOUNT_SID" = "null" ]; then
    echo -e "${RED}❌ ERROR: account_sid not found in config${NC}"
    exit 1
fi

if [ -z "$WHATSAPP_NUMBER" ] || [ "$WHATSAPP_NUMBER" = "null" ]; then
    echo -e "${RED}❌ ERROR: whatsapp_number not found in config${NC}"
    exit 1
fi

echo -e "${GREEN}✅ Configuration loaded successfully${NC}"
echo "   Account SID: ${ACCOUNT_SID:0:15}..."
echo "   Auth Token: ${AUTH_TOKEN:0:15}..."
echo "   Sender: $WHATSAPP_NUMBER"
echo "   gRPC Port: $GRPC_PORT"
echo ""

# Check if service is running
echo "${YELLOW}2. Checking if Twilio WhatsApp service is running...${NC}"
if curl -s http://localhost:$METRICS_PORT/health > /dev/null 2>&1; then
    echo -e "${GREEN}✅ Service is running${NC}"
else
    echo -e "${RED}❌ Service not running!${NC}"
    echo "   Start it with: make run-twilio-whatsapp"
    exit 1
fi
echo ""

# Test gRPC connection
echo "${YELLOW}3. Testing gRPC connection...${NC}"
if timeout 2 bash -c "echo '' > /dev/tcp/localhost/$GRPC_PORT" 2>/dev/null; then
    echo -e "${GREEN}✅ gRPC port $GRPC_PORT is open${NC}"
else
    echo -e "${RED}❌ Cannot connect to gRPC port $GRPC_PORT${NC}"
    exit 1
fi
echo ""

# Summary
echo "${YELLOW}4. Sandbox Status Check${NC}"
echo ""
echo "   📌 Sender (your Twilio number): $WHATSAPP_NUMBER"
echo "   📌 Account SID: ${ACCOUNT_SID:0:20}..."
echo "   📌 gRPC Endpoint: localhost:$GRPC_PORT"
echo ""
echo -e "${YELLOW}Required actions:${NC}"
echo "   1. ☑️  Number is in SANDBOX PARTICIPANT LIST"
echo "      → Go to: https://console.twilio.com/us/account/sms/whatsapp/learn"
echo ""
echo "   2. ☑️  Reply to Twilio's WhatsApp message with verification code"
echo "      → You should receive: 'Twilio Sandbox Participant Verification'"  
echo ""
echo "   3. ☑️  Use E.164 format when testing"
echo "      → Example: +6285173207755"
echo ""
echo "   4. ☑️  Wait 5-30 seconds for sandbox messages to deliver"
echo "      → Sandbox is slower than production"
echo ""

# Send test message
echo "${YELLOW}5. Sending test message...${NC}"
echo ""

# Use grpcurl if available, otherwise use Go test
if command -v grpcurl &> /dev/null; then
    echo "   Recipient number: "
    read -p "   Enter your verified sandbox number (e.g., +6285173207755): " RECIPIENT
    
    if [ -z "$RECIPIENT" ]; then
        echo -e "${RED}❌ Number required${NC}"
        exit 1
    fi
    
    echo ""
    echo "   Sending: 'Hello from Twilio WhatsApp Sandbox Test!'"
    grpcurl -plaintext \
        -d "{\"to\": \"$RECIPIENT\", \"body\": \"Hello from Twilio WhatsApp Sandbox Test! Sent at $(date '+%Y-%m-%d %H:%M:%S')\"}" \
        localhost:$GRPC_PORT proto.TwilioWhatsAppService/SendMessage
else
    echo "   ⚠️  grpcurl not installed, running Go test instead..."
    go test -v -run TestSendMessage ./tests/twilio/ -timeout 10s
fi

echo ""
echo "${GREEN}✅ Test complete!${NC}"
echo ""
echo "${YELLOW}Next steps:${NC}"
echo "   • Check your WhatsApp inbox (may take 5-30 seconds)"
echo "   • Check Twilio logs: https://console.twilio.com/us/account/logs"
echo "   • If no message: Verify number is in sandbox participants"
echo ""
