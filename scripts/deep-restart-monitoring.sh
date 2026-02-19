#!/bin/bash

# Service Platform Monitoring Deep Restart Script
# Clears Grafana cache and restarts monitoring services

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
CONFIG_FILE="${PROJECT_ROOT}/internal/config/service-platform.${CONFIG_MODE}.yaml"
if [ ! -f "$CONFIG_FILE" ]; then
    echo "❌ Config file not found: $CONFIG_FILE"
    exit 1
fi

# Extract monitoring settings
PROMETHEUS_PORT=$(yq -r '.metrics.api_port' "$CONFIG_FILE")
GRAFANA_PORT=$(yq -r '.metrics.grafana_port' "$CONFIG_FILE")
LT_PORT=$(yq -r '.libretranslate.port' "$CONFIG_FILE")

echo "🔄 Deep restarting Service Platform monitoring..."

# Update prometheus.yml with actual ports
sed -i "s/__LT_PORT__/$LT_PORT/g" "${PROJECT_ROOT}/prometheus.yml"

# Detect container runtime
if command -v podman &> /dev/null; then
    CONTAINER_CMD="podman"
    COMPOSE_CMD="podman-compose"
    echo "📦 Using Podman"
elif command -v docker &> /dev/null; then
    CONTAINER_CMD="docker"
    COMPOSE_CMD="docker-compose"
    echo "📦 Using Docker"
else
    echo "❌ Error: Neither Podman nor Docker is installed"
    exit 1
fi

echo ""
echo "🛑 Stopping monitoring services..."
$COMPOSE_CMD -f docker/docker-compose.monitoring.yml down

echo ""
echo "🗑️  Removing Grafana cache volume..."
$CONTAINER_CMD volume rm service-platform_grafana_data 2>/dev/null || echo "   (Volume already removed or doesn't exist)"

echo ""
echo "🚀 Starting monitoring services with fresh Grafana..."
export PROMETHEUS_PORT GRAFANA_PORT LT_PORT
$COMPOSE_CMD -f docker/docker-compose.monitoring.yml up -d

echo ""
echo "⏳ Waiting for services to be ready..."
sleep 5

# Check if services are running
if $CONTAINER_CMD ps --filter "label=io.podman.compose.project=service-platform" --format "{{.Names}}" | grep -q "service-platform-grafana"; then
    echo "✅ Grafana started successfully"
elif $CONTAINER_CMD ps --filter "label=com.docker.compose.project=service-platform" --format "{{.Names}}" | grep -q "service-platform-grafana"; then
    echo "✅ Grafana started successfully"
else
    echo "⚠️  Warning: Grafana may not have started properly"
fi

if $CONTAINER_CMD ps --filter "label=io.podman.compose.project=service-platform" --format "{{.Names}}" | grep -q "service-platform-prometheus"; then
    echo "✅ Prometheus started successfully"
elif $CONTAINER_CMD ps --filter "label=com.docker.compose.project=service-platform" --format "{{.Names}}" | grep -q "service-platform-prometheus"; then
    echo "✅ Prometheus started successfully"
else
    echo "⚠️  Warning: Prometheus may not have started properly"
fi

echo ""
echo "🎉 Deep restart completed!"
echo ""
echo "📊 Service URLs:"
echo "   Grafana:    http://localhost:${GRAFANA_PORT}"
echo "   Prometheus: http://localhost:${PROMETHEUS_PORT}"
echo "   LibreTranslate: http://localhost:${LT_PORT}"
echo ""
echo "💡 Note: Grafana cache has been cleared. All dashboards will be loaded fresh from JSON files."
echo "   You may need to wait 10-15 seconds for Grafana to fully initialize."
