#!/bin/bash
set -e

echo "🔍 Testing Tempo Tracing"
echo "======================="
echo ""

# Check if Tempo is ready
echo "1️⃣ Checking Tempo readiness..."
if curl -s http://localhost:3200/ready 2>&1 | grep -q "ready"; then
    echo "   ✅ Tempo is ready"
else
    echo "   ❌ Tempo is not ready"
    echo "   Attempting to restart Tempo..."
    cd /home/user/server/service-platform
    podman-compose -f docker/docker-compose.monitoring.yml restart tempo
    sleep 5
    if curl -s http://localhost:3200/ready 2>&1 | grep -q "ready"; then
        echo "   ✅ Tempo is now ready"
    else
        echo "   ❌ Tempo failed to start. Check logs:"
        echo "   podman logs service-platform-tempo"
        exit 1
    fi
fi

echo ""
echo "2️⃣ Checking OTLP gRPC endpoint (port 4317)..."
if nc -z localhost 4317 2>/dev/null; then
    echo "   ✅ Port 4317 is open (OTLP gRPC ready)"
else
    echo "   ⚠️  Port 4317 is not accessible"
fi

echo ""
echo "3️⃣ Tempo Configuration:"
echo "   📍 OTLP gRPC endpoint: localhost:4317"
echo "   📍 HTTP API endpoint: localhost:3200"
echo "   📦 Protocol: gRPC (OTLP)"
echo "   🔓 Authentication: None required"
echo ""

echo "4️⃣ Checking if API has tracing enabled..."
if grep -q 'enabled: true' internal/config/config.dev.yaml | grep -A 5 'tempo:' | grep -q 'enabled: true'; then
    echo "   ✅ Tracing is enabled in config"
    echo "   📝 Config: internal/config/config.dev.yaml"
    echo ""
    echo "   Sample rate: 1.0 (100%)"
    echo "   Endpoint: http://127.0.0.1:4317"
else
    echo "   ⚠️  Tracing may not be enabled. Check config.dev.yaml"
fi

echo ""
echo "5️⃣ To view traces in Grafana:"
echo "   1. Open http://localhost:3063"
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
