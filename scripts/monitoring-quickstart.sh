#!/bin/bash

# Quick Start Script for Loki + Tempo + Grafana Monitoring Stack
# Usage: bash scripts/monitoring-quickstart.sh [action]
# 
# This script prioritizes Podman Compose over Docker Compose
# If podman-compose is not available, it will fall back to docker-compose

ACTION=${1:-start}

# Determine which container runtime to use (prioritize podman-compose)
if command -v podman-compose &> /dev/null; then
    COMPOSE_CMD="podman-compose"
    RUNTIME="Podman"
elif command -v docker-compose &> /dev/null; then
    COMPOSE_CMD="docker-compose"
    RUNTIME="Docker"
else
    echo "❌ Error: Neither 'podman-compose' nor 'docker-compose' found!"
    echo "Please install Podman Compose or Docker Compose."
    exit 1
fi

case $ACTION in
  start)
    echo "🚀 Starting monitoring stack using $RUNTIME (Prometheus, Grafana, Loki, Tempo, Nginx)..."
    $COMPOSE_CMD -f docker-compose.monitoring.yml up -d
    echo ""
    echo "✅ Monitoring stack started!"
    echo ""
    echo "📊 Access Points:"
    echo "  • Grafana (direct): http://localhost:3063"
    echo "  • Grafana (via Nginx auth): http://localhost:9180"
    echo "  • Prometheus: http://localhost:9090"
    echo "  • Loki API: http://localhost:3100"
    echo "  • Tempo API: http://localhost:3200"
    echo ""
    echo "🔐 Nginx Credentials (if using port 9180):"
    echo "  • Username: admin"
    echo "  • Password: Net55206011##"
    echo ""
    echo "ℹ️  Container Runtime: $RUNTIME"
    echo ""
    ;;

  stop)
    echo "⛔ Stopping monitoring stack..."
    $COMPOSE_CMD -f docker-compose.monitoring.yml down
    echo "✅ Monitoring stack stopped!"
    ;;

  restart)
    echo "🔄 Restarting monitoring stack..."
    $COMPOSE_CMD -f docker-compose.monitoring.yml down
    echo "⏳ Waiting a moment before restarting..."
    sleep 4
    $COMPOSE_CMD -f docker-compose.monitoring.yml up -d
    echo "✅ Monitoring stack restarted!"
    ;;

  logs)
    SERVICE=${2:-}
    if [ -z "$SERVICE" ]; then
      echo "📋 Showing logs for all services..."
      $COMPOSE_CMD -f docker-compose.monitoring.yml logs -f
    else
      echo "📋 Showing logs for $SERVICE..."
      $COMPOSE_CMD -f docker-compose.monitoring.yml logs -f "$SERVICE"
    fi
    ;;

  status)
    echo "📊 Monitoring Stack Status (using $RUNTIME):"
    $COMPOSE_CMD -f docker-compose.monitoring.yml ps
    ;;

  ps)
    $COMPOSE_CMD -f docker-compose.monitoring.yml ps
    ;;

  rebuild)
    echo "🔨 Rebuilding monitoring images..."
    $COMPOSE_CMD -f docker-compose.monitoring.yml build --no-cache
    echo "✅ Rebuild complete!"
    ;;

  clean)
    echo "🧹 Cleaning up monitoring stack (volumes will be removed)..."
    $COMPOSE_CMD -f docker-compose.monitoring.yml down -v
    echo "✅ Cleanup complete!"
    ;;

  test-loki)
    echo "🧪 Testing Loki connectivity..."
    echo ""
    echo "Testing Loki API endpoints:"
    echo "1. Labels endpoint:"
    curl -s http://localhost:3100/loki/api/v1/labels | jq '.'
    echo ""
    echo "2. Query for service-platform logs:"
    curl -s -G http://localhost:3100/loki/api/v1/query --data-urlencode 'query={service="service-platform"}' | jq '.data.result | length'
    echo ""
    ;;

  test-loki-push)
    echo "🧪 Testing Loki push API..."
    PAYLOAD='{
      "streams": [
        {
          "labels": {
            "service": "test",
            "test": "true"
          },
          "entries": [
            {
              "ts": "'$(date +%s)'000000000",
              "line": "Test log from curl"
            }
          ]
        }
      ]
    }'
    RESPONSE=$(curl -s -X POST "http://127.0.0.1:3100/loki/api/v1/push" \
      -H "Content-Type: application/json" \
      -d "$PAYLOAD" -w "\n%{http_code}")
    echo "Response: $RESPONSE"
    ;;

  test-tempo)
    echo "🧪 Testing Tempo connectivity..."
    curl -v http://localhost:3200/tempo/api/search
    echo ""
    ;;

  test-nginx)
    echo "🧪 Testing Nginx health check..."
    curl -v http://localhost:8080/health
    echo ""
    ;;

  change-password)
    echo "🔐 Changing Nginx Basic Auth Password..."
    echo "Generate new password hash:"
    openssl passwd -apr1
    echo ""
    echo "Edit monitoring/nginx/.htpasswd with the new hash"
    echo "Then restart nginx: $COMPOSE_CMD -f docker-compose.monitoring.yml restart nginx-auth"
    ;;

  help)
    echo "📖 Monitoring Stack Control Script"
    echo ""
    echo "Usage: bash scripts/monitoring-quickstart.sh [action]"
    echo ""
    echo "Actions:"
    echo "  start              - Start the monitoring stack (uses Podman or Docker)"
    echo "  stop               - Stop the monitoring stack"
    echo "  restart            - Restart the monitoring stack"
    echo "  status, ps         - Show status of services"
    echo "  logs [service]     - Show logs (all or specific service)"
    echo "  rebuild            - Rebuild container images"
    echo "  clean              - Remove all containers and volumes"
    echo "  test-loki          - Test Loki connectivity"
    echo "  test-tempo         - Test Tempo connectivity"
    echo "  change-password    - Change Nginx basic auth credentials"
    echo "  help               - Show this help message"
    echo ""
    echo "Examples:"
    echo "  bash scripts/monitoring-quickstart.sh start"
    echo "  bash scripts/monitoring-quickstart.sh logs loki"
    echo "  bash scripts/monitoring-quickstart.sh status"
    echo ""
    echo "📍 Access Points:"
    echo "  • Grafana (direct): http://localhost:3063"
    echo "  • Grafana (via Nginx): http://localhost:9180"
    echo "  • Prometheus: http://localhost:9090"
    echo ""
    echo "🔐 Default Credentials: admin / Net55206011##"
    echo "ℹ️  Current Runtime: $RUNTIME"
    ;;

  *)
    echo "❌ Unknown action: $ACTION"
    echo "Use 'bash scripts/monitoring-quickstart.sh help' for available commands"
    exit 1
    ;;
esac
