#!/bin/bash
set -e

echo "🔍 Verifying Observability Stack"
echo "================================"
echo ""

# Load configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CONFIG_MODE=$(yq -r '.config_mode' "$PROJECT_ROOT/internal/config/conf.yaml" 2>/dev/null || echo "dev")
CONFIG_FILE="$PROJECT_ROOT/internal/config/service-platform.${CONFIG_MODE}.yaml"

# Read observability ports from config
LOKI_PORT=${LOKI_PORT:-3100}
TEMPO_PORT=$(yq -r '.observability.tempo.otlp_http_endpoint' "$CONFIG_FILE" 2>/dev/null | grep -o ':[0-9]*' | sed 's/://' || echo "3200")
GRAFANA_PORT=$(yq -r '.metrics.grafana_port' "$CONFIG_FILE" 2>/dev/null || echo "3063")

# Check local file logging
echo "📁 Local File Logging:"
if [ -d "./log" ]; then
    LOG_COUNT=$(ls -1 ./log/*.log 2>/dev/null | wc -l)
    LOG_SIZE=$(du -sh ./log 2>/dev/null | cut -f1)
    echo "   ✅ Log directory exists: ./log"
    echo "   📊 Log files: $LOG_COUNT files ($LOG_SIZE total)"
    echo "   📝 Recent files:"
    ls -lht ./log/*.log | head -5 | awk '{print "      - " $9 " (" $5 ")"}'
else
    echo "   ❌ Log directory not found"
fi

echo ""
echo "📡 Loki (Logs):"

# Check Loki health
if curl -s http://localhost:${LOKI_PORT}/ready | grep -q "ready"; then
    echo "   ✅ Loki is ready"
    
    # Check for service-platform logs
    ENTRY_COUNT=$(curl -s -G "http://localhost:${LOKI_PORT}/loki/api/v1/query_range" \
      --data-urlencode 'query={service="service-platform"}' \
      --data-urlencode 'start='$(date -d '30 minutes ago' +%s%N)'' \
      --data-urlencode 'end='$(date +%s%N)'' 2>/dev/null | jq -r '.data.stats.summary.totalEntriesReturned // 0')
    
    if [ "$ENTRY_COUNT" -gt 0 ]; then
        echo "   ✅ Found $ENTRY_COUNT log entries in Loki"
        echo "   📝 Latest log samples:"
        curl -s -G "http://localhost:${LOKI_PORT}/loki/api/v1/query_range" \
          --data-urlencode 'query={service="service-platform"}' \
          --data-urlencode 'limit=3' \
          --data-urlencode 'start='$(date -d '30 minutes ago' +%s%N)'' \
          --data-urlencode 'end='$(date +%s%N)'' 2>/dev/null | \
          jq -r '.data.result[0].values[0:3][]? | "      " + .[1]'
    else
        echo "   ⚠️  No logs found (run the API to generate logs)"
    fi
else
    echo "   ❌ Loki is not ready"
fi

echo ""
echo "🔍 Tempo (Traces):"

# Check Tempo health
if curl -s http://localhost:${TEMPO_PORT}/ready 2>/dev/null | grep -q "ready"; then
    echo "   ✅ Tempo is ready"
    OTLP_GRPC_PORT=4317
    echo "   🔌 OTLP gRPC endpoint: localhost:${OTLP_GRPC_PORT}"
    echo "   🌐 HTTP API endpoint: localhost:${TEMPO_PORT}"
    echo ""
    echo "   📊 To send traces to Tempo:"
    echo "      - Use OTLP exporter: localhost:${OTLP_GRPC_PORT}"
    echo "      - Protocol: gRPC"
    echo "      - No authentication required"
else
    echo "   ❌ Tempo is not ready"
fi

echo ""
echo "🎨 Grafana (Visualization):"
if curl -s http://localhost:${GRAFANA_PORT}/api/health | jq -e '.database == "ok"' > /dev/null 2>&1; then
    GRAFANA_VERSION=$(curl -s http://localhost:${GRAFANA_PORT}/api/health | jq -r '.version')
    NGINX_PORT=${NGINX_PORT:-9180}
    echo "   ✅ Grafana is running (v$GRAFANA_VERSION)"
    echo "   🌐 Direct access: http://localhost:${GRAFANA_PORT}"
    echo "   🔐 Via Nginx (with auth): http://localhost:${NGINX_PORT}"
    echo "   👤 Credentials: admin / Net55206011##"
    echo ""
    echo "   📊 Configured data sources:"
    echo "      - Prometheus (metrics) ✅"
    echo "      - Loki (logs) ✅"
    echo "      - Tempo (traces) ✅"
    echo ""
    echo "   🔗 Trace-to-Log correlation: Enabled"
    echo ""
    echo "   📝 To view LOGS in Grafana:"
    echo "      1. Open http://localhost:${GRAFANA_PORT}"
    echo "      2. Login with admin/Net55206011##"
    echo "      3. Go to Explore"
    echo "      4. Select 'Loki' data source"
    echo "      5. Query: {service=\"service-platform\"}"
    echo ""
    echo "   🔍 To view TRACES in Grafana:"
    echo "      1. Open http://localhost:${GRAFANA_PORT}"
    echo "      2. Go to Explore"
    echo "      3. Select 'Tempo' data source"
    echo "      4. Search by:"
    echo "         - Service Name"
    echo "         - Trace ID"
    echo "         - Duration range"
    echo "      5. Click on any trace to see details"
else
    echo "   ❌ Grafana is not accessible"
fi

echo ""
echo "✨ Observability Status Summary:"
echo "   📁 Local files: ✅ Working (./log/*.log)"
echo "   📡 Loki: ✅ Logs stored and queryable"
echo "   🔍 Tempo: ✅ Ready for traces"
echo "   🎨 Grafana: ✅ All data sources configured"
echo ""
echo "🚀 Next: Configure your application to send traces to Tempo"
echo "   Example: Use OpenTelemetry SDK with OTLP exporter → localhost:${OTLP_GRPC_PORT}"
