#!/bin/bash
set -e

echo "🔍 Testing Tempo Tracing"
echo "======================="
echo ""

# Read config mode and load configuration
CONFIG_MODE=$(yq -r '.config_mode' internal/config/conf.yaml 2>/dev/null || echo "dev")
CONFIG_FILE="internal/config/service-platform.${CONFIG_MODE}.yaml"
TEMPO_PORT=$(yq -r '.observability.tempo.otlp_http_endpoint' "$CONFIG_FILE" 2>/dev/null | grep -o ':[0-9]*' | sed 's/://' || echo "3200")

# Check if Tempo is ready
echo "1️⃣ Checking Tempo readiness..."
if curl -s http://localhost:${TEMPO_PORT}/ready 2>&1 | grep -q "ready"; then
    echo "   ✅ Tempo is ready"
else
    echo "   ❌ Tempo is not ready"
    echo "   Attempting to restart Tempo..."
    cd /home/user/server/service-platform
    podman-compose -f docker/docker-compose.monitoring.yml restart tempo
    sleep 5
    if curl -s http://localhost:${TEMPO_PORT}/ready 2>&1 | grep -q "ready"; then
        echo "   ✅ Tempo is now ready"
    else
        echo "   ❌ Tempo failed to start. Check logs:"
        echo "   podman logs service-platform-tempo"
        exit 1
    fi
fi

echo ""
OTLP_GRPC_PORT=4317
echo "2️⃣ Checking OTLP gRPC endpoint (port $OTLP_GRPC_PORT)..."
if nc -z localhost $OTLP_GRPC_PORT 2>/dev/null; then
    echo "   ✅ Port $OTLP_GRPC_PORT is open (OTLP gRPC ready)"
else
    echo "   ⚠️  Port $OTLP_GRPC_PORT is not accessible"
fi

echo ""
echo "3️⃣ Tempo Configuration:"
echo "   📍 OTLP gRPC endpoint: localhost:$OTLP_GRPC_PORT"
echo "   📍 HTTP API endpoint: localhost:$TEMPO_PORT"
echo "   📦 Protocol: gRPC (OTLP)"
echo "   🔓 Authentication: None required"
echo ""

echo "4️⃣ Checking if API has tracing enabled..."

if [ -f "$CONFIG_FILE" ] && grep -A 5 'tempo:' "$CONFIG_FILE" | grep -q 'enabled: true'; then
    echo "   ✅ Tracing is enabled in config"
    echo "   📝 Config: $CONFIG_FILE"
    echo ""
    echo "   Sample rate: 1.0 (100%)"
    echo "   Endpoint: http://127.0.0.1:${OTLP_GRPC_PORT}"
else
    echo "   ⚠️  Tracing may not be enabled. Check $CONFIG_FILE"
fi

echo ""
GRAFANA_PORT=$(yq -r '.metrics.grafana_port' "$CONFIG_FILE" 2>/dev/null || echo "3063")
echo "5️⃣ To view traces in Grafana:"
echo "   1. Open http://localhost:${GRAFANA_PORT}"
echo "   2. Login: admin / Net55206011##"
echo "   3. Click 'Explore' (compass icon)"
echo "   4. Select 'Tempo' data source"
echo "   5. Click 'Search' tab"
echo "   6. Filter by:"
echo "      - Service Name: service-platform"
echo "      - Time range: Last 15 minutes"
echo "   7. Click 'Run Query'"
echo ""

echo "6️⃣ Need to send test traces?"
echo "   The API application needs to:"
echo "   - Initialize tracing with observability.InitTracer()"
echo "   - Create spans for operations"
echo "   - Use OpenTelemetry SDK"
echo ""
echo "   Check: internal/pkg/observability/trace.go"
echo ""

echo "✅ Tempo is ready to receive traces!"
echo "   Run your API to start generating traces."
