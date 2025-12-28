#!/bin/bash

# Service Platform Monitoring Stop Script
# Reads configuration from YAML files based on config_mode

set -e

SCRIPT_DIR="$(dirname "$0")"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Read config mode from conf.yaml
CONFIG_MODE=$(yq -r '.config_mode' "${PROJECT_ROOT}/internal/config/conf.yaml")
if [ "$CONFIG_MODE" != "dev" ] && [ "$CONFIG_MODE" != "prod" ]; then
    echo "❌ Invalid config_mode: $CONFIG_MODE. Must be 'dev' or 'prod'"
    exit 1
fi

echo "🔧 Using configuration: $CONFIG_MODE"

# Load monitoring config from appropriate file
CONFIG_FILE="${PROJECT_ROOT}/internal/config/config.${CONFIG_MODE}.yaml"
if [ ! -f "$CONFIG_FILE" ]; then
    echo "❌ Config file not found: $CONFIG_FILE"
    exit 1
fi

# Extract monitoring settings
PROMETHEUS_PORT=$(yq -r '.metrics.api_port' "$CONFIG_FILE")
GRAFANA_PORT=3030  # Fixed port for Grafana UI

COMPOSE_FILE="${PROJECT_ROOT}/docker-compose.monitoring.yml"

echo "🛑 Stopping Service Platform Monitoring..."

# Check runtime preference: Podman first, then Docker
if command -v podman-compose &> /dev/null; then
    echo "📦 Using Podman Compose"
    RUNTIME="podman-compose"

    PROMETHEUS_PORT=$PROMETHEUS_PORT GRAFANA_PORT=$GRAFANA_PORT podman-compose -f "${COMPOSE_FILE}" down -v

elif command -v docker-compose &> /dev/null && command -v docker &> /dev/null; then
    echo "🐳 Using Docker Compose"
    RUNTIME="docker-compose"

    PROMETHEUS_PORT=$PROMETHEUS_PORT GRAFANA_PORT=$GRAFANA_PORT docker-compose -f "${COMPOSE_FILE}" down -v

else
    echo "❌ Neither podman-compose nor docker-compose found."
    exit 1
fi

echo "✅ Monitoring services stopped with ${RUNTIME}!"