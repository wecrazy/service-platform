#!/bin/bash

# N8N Clear Workflows Script
# Removes all workflows from the N8N instance via API
# Reads configuration from config.dev.yaml or config.prod.yaml based on conf.yaml
# Usage: bash scripts/n8n-clear-workflows.sh [--force]

# Detect script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Check if yq is installed
if ! command -v yq &> /dev/null; then
	echo "❌ yq is not installed (required to parse YAML)"
	echo "Install with: apt-get install yq (or brew install yq on macOS)"
	exit 1
fi

# Read config mode from conf.yaml
CONFIG_MODE=$(yq -r '.config_mode' "${PROJECT_ROOT}/internal/config/conf.yaml")
if [ "$CONFIG_MODE" != "dev" ] && [ "$CONFIG_MODE" != "prod" ]; then
	echo "❌ Invalid config_mode: $CONFIG_MODE. Must be 'dev' or 'prod'"
	exit 1
fi

# Load config from appropriate file
CONFIG_FILE="${PROJECT_ROOT}/internal/config/config.${CONFIG_MODE}.yaml"
if [ ! -f "$CONFIG_FILE" ]; then
	echo "❌ Config file not found: $CONFIG_FILE"
	exit 1
fi

# Read from config file
N8N_HOST=$(yq -r '.n8n.host' "$CONFIG_FILE")
N8N_PORT=$(yq -r '.n8n.port' "$CONFIG_FILE")
N8N_API_KEY=$(yq -r '.n8n.api_key' "$CONFIG_FILE")

# Allow environment variable override
N8N_HOST="${N8N_HOST:-localhost}"
N8N_BASE_URL="http://$N8N_HOST:$N8N_PORT/api/v1"

FORCE_DELETE="${1:-}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🧹 N8N Workflow Cleaner${NC}"
echo "=================================="
echo "Base URL: $N8N_BASE_URL"
echo ""

# Check if jq is installed
if ! command -v jq &> /dev/null; then
echo -e "${RED}❌ jq is not installed${NC}"
echo "Install it with: apt-get install jq (or brew install jq on macOS)"
exit 1
fi

# Check if N8N is running
echo -e "${YELLOW}⏳ Checking if N8N is running...${NC}"
HEALTH_URL="http://$N8N_HOST:$N8N_PORT/health"
HEALTH_RESPONSE=$(curl -s -w "\n%{http_code}" "$HEALTH_URL" 2>/dev/null)
HTTP_CODE=$(echo "$HEALTH_RESPONSE" | tail -n 1)

if [ "$HTTP_CODE" != "200" ]; then
echo -e "${RED}❌ N8N is not running at http://$N8N_HOST:$N8N_PORT (HTTP $HTTP_CODE)${NC}"
echo ""
echo "Checking if N8N container is running:"
podman ps | grep n8n || echo "  No N8N container found"
echo ""
echo "Please start N8N with: make run-n8n"
exit 1
fi
echo -e "${GREEN}✅ N8N is running${NC}"
echo ""

# Build curl command with optional API key
CURL_CMD="curl -s -X GET"
if [ -n "$N8N_API_KEY" ]; then
CURL_CMD="$CURL_CMD -H \"X-N8N-API-KEY: $N8N_API_KEY\""
fi

# Get all workflows
echo -e "${YELLOW}📋 Fetching all workflows...${NC}"
WORKFLOWS_RESPONSE=$(eval "$CURL_CMD $N8N_BASE_URL/workflows?limit=250" 2>/dev/null)

# Check for API key error
if echo "$WORKFLOWS_RESPONSE" | grep -q "API-KEY"; then
echo -e "${YELLOW}⚠️  N8N requires API key${NC}"
echo "Set N8N_API_KEY environment variable:"
echo "  export N8N_API_KEY='your-api-key'"
echo "  make n8n-clear"
exit 1
fi

# Parse workflows with error handling
WORKFLOWS=$(echo "$WORKFLOWS_RESPONSE" | jq -r '.data[] | "\(.id):\(.name)"' 2>/dev/null) || {
echo -e "${RED}❌ Failed to parse workflows response${NC}"
echo "Response: $WORKFLOWS_RESPONSE"
exit 1
}

if [ -z "$WORKFLOWS" ]; then
echo -e "${GREEN}✅ No workflows found${NC}"
exit 0
fi

# Count workflows
WORKFLOW_COUNT=$(echo "$WORKFLOWS" | grep -c ":")
echo -e "${BLUE}Found $WORKFLOW_COUNT workflow(s):${NC}"
echo ""

# Create indexed arrays for IDs and names
declare -a WORKFLOW_IDS
declare -a WORKFLOW_NAMES
INDEX=1

while IFS=':' read -r ID NAME; do
	if [ -n "$ID" ] && [ -n "$NAME" ]; then
		WORKFLOW_IDS[$INDEX]="$ID"
		WORKFLOW_NAMES[$INDEX]="$NAME"
		echo -e "  ${BLUE}[$INDEX]${NC} ${YELLOW}[$ID]${NC} $NAME"
		((INDEX++))
	fi
done <<< "$WORKFLOWS"

echo ""

# Get user selection
if [ "$FORCE_DELETE" = "--force" ]; then
	SELECTED_INDICES=$(seq 1 $((INDEX-1)))
	echo "Force delete selected - deleting all workflows"
else
	echo "Enter workflow numbers to delete (e.g., '1 3 5') or 'all' for all workflows:"
	echo -n "Selection: "
	read -r SELECTION
	
	if [ "$SELECTION" = "all" ]; then
		SELECTED_INDICES=$(seq 1 $((INDEX-1)))
	elif [ -z "$SELECTION" ]; then
		echo -e "${YELLOW}❌ No selection made, cancelling${NC}"
		exit 0
	else
		SELECTED_INDICES="$SELECTION"
	fi
fi

echo ""

# Confirmation
echo -e "${RED}⚠️  WARNING: This will delete the selected workflows!${NC}"
echo "Workflows to delete:"
for NUM in $SELECTED_INDICES; do
	if [ -n "${WORKFLOW_NAMES[$NUM]}" ]; then
		echo -e "  ${YELLOW}[$NUM]${NC} ${WORKFLOW_NAMES[$NUM]}"
	fi
done
echo ""

if [ "$FORCE_DELETE" != "--force" ]; then
	read -p "Type 'yes' to confirm: " CONFIRM
	if [ "$CONFIRM" != "yes" ]; then
		echo -e "${YELLOW}❌ Cancelled${NC}"
		exit 0
	fi
fi

# Delete selected workflows
echo ""
echo -e "${YELLOW}🗑️  Deleting workflows...${NC}"
DELETED=0
FAILED=0

for NUM in $SELECTED_INDICES; do
	if [ -n "${WORKFLOW_IDS[$NUM]}" ]; then
		ID="${WORKFLOW_IDS[$NUM]}"
		NAME="${WORKFLOW_NAMES[$NUM]}"
		echo -n "  Deleting [$NUM] $NAME... "

		DELETE_CMD="curl -s -w \"%{http_code}\" -X DELETE"
		if [ -n "$N8N_API_KEY" ] && [ "$N8N_API_KEY" != "null" ]; then
			DELETE_CMD="$DELETE_CMD -H \"X-N8N-API-KEY: $N8N_API_KEY\""
		fi
		DELETE_CMD="$DELETE_CMD $N8N_BASE_URL/workflows/$ID -o /dev/null"

		HTTP_CODE=$(eval "$DELETE_CMD" 2>/dev/null || true)

		if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "204" ]; then
			echo -e "${GREEN}✅ Deleted${NC}"
			((DELETED++))
		else
			echo -e "${RED}❌ Failed (HTTP $HTTP_CODE)${NC}"
			((FAILED++))
		fi
	fi
done

echo ""
echo "=================================="
echo -e "${GREEN}✅ Deleted: $DELETED${NC}"
if [ $FAILED -gt 0 ]; then
echo -e "${RED}❌ Failed: $FAILED${NC}"
fi
echo ""
echo -e "${GREEN}🎉 Workflow cleanup complete!${NC}"
