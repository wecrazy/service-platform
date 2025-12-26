#!/bin/bash

# Check if an argument is provided
if [ -z "$1" ]; then
    echo "Usage: $0 [dev|prod]"
    exit 1
fi

MODE=$1
CONFIG_DIR="internal/config"
CONF_FILE="$CONFIG_DIR/conf.yaml"
TARGET_CONFIG="$CONFIG_DIR/config.$MODE.yaml"

# Validate mode
if [[ "$MODE" != "dev" && "$MODE" != "prod" ]]; then
    echo "Error: Invalid mode. Use 'dev' or 'prod'."
    exit 1
fi

# Check if the target config file exists
if [ ! -f "$TARGET_CONFIG" ]; then
    echo "Warning: $TARGET_CONFIG does not exist."
    if [ "$MODE" == "prod" ] && [ -f "$CONFIG_DIR/config.dev.yaml" ]; then
        echo "Creating $TARGET_CONFIG from config.dev.yaml..."
        cp "$CONFIG_DIR/config.dev.yaml" "$TARGET_CONFIG"
    else
        echo "Please create it before running the application."
    fi
fi

# Update conf.yaml
# We use a temporary file to ensure atomic write and handle different sed versions
if [ -f "$CONF_FILE" ]; then
    # Create content with new mode
    echo "config_mode: \"$MODE\" # dev | prod" > "$CONF_FILE"
    echo "Switched configuration to '$MODE' mode."
else
    echo "Error: $CONF_FILE not found."
    exit 1
fi
