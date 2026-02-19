#!/bin/bash

# Switch config_mode in conf.yaml
# Usage: ./switch_config.sh dev|prod

MODE=$1

# Validate argument
if [ -z "$MODE" ]; then
    echo "Usage: $0 [dev|prod]"
    echo ""
    echo "Examples:"
    echo "  $0 dev"
    echo "  $0 prod"
    exit 1
fi

# Validate mode
if [[ "$MODE" != "dev" && "$MODE" != "prod" ]]; then
    echo "❌ Error: Invalid mode. Use 'dev' or 'prod'."
    exit 1
fi

CONFIG_DIR="internal/config"
CONF_FILE="$CONFIG_DIR/conf.yaml"

# Check if conf.yaml exists
if [ ! -f "$CONF_FILE" ]; then
    echo "❌ Error: $CONF_FILE not found."
    exit 1
fi

# Update conf.yaml with new mode using sed
sed -i "s/^config_mode:.*/config_mode: \"$MODE\" # dev | prod/" "$CONF_FILE"

echo "✅ Switched config_mode to '$MODE'"
echo "📝 Updated: $CONF_FILE"
