#!/bin/bash

# Script to switch between dev and prod config modes

CONFIG_FILE="config/conf.yaml"

if [ ! -f "$CONFIG_FILE" ]; then
    echo "Error: Config file '$CONFIG_FILE' not found!"
    exit 1
fi

echo "Current CONFIG_MODE:"
grep "^CONFIG_MODE:" "$CONFIG_FILE"

echo ""
echo "Select mode:"
echo "1) Development (dev)"
echo "2) Production (prod)"
read -p "Enter choice (1 or 2): " choice

case $choice in
    1)
        sed -i 's/CONFIG_MODE: "prod"/CONFIG_MODE: "dev"/g' "$CONFIG_FILE"
        echo "✅ Switched to development mode"
        ;;
    2)
        sed -i 's/CONFIG_MODE: "dev"/CONFIG_MODE: "prod"/g' "$CONFIG_FILE"
        echo "✅ Switched to PRODUCTION mode"
        ;;
    *)
        echo "❌ Invalid choice. Exiting."
        exit 1
        ;;
esac

echo ""
echo "New CONFIG_MODE:"
grep "^CONFIG_MODE:" "$CONFIG_FILE"