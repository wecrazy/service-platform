#!/bin/bash

# Comprehensive Health Check Script for Service Platform
# Checks container runtime, services, monitoring stack, database, and health endpoints
# Usage: bash scripts/health-check-all.sh

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration - read from config files
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Determine config mode and load config
CONFIG_MODE=$(yq '.config_mode' "$PROJECT_ROOT/internal/config/conf.yaml" 2>/dev/null || echo "dev")
CONFIG_FILE="$PROJECT_ROOT/internal/config/config.${CONFIG_MODE}.yaml"

# Read ports from config
API_PORT=$(yq '.app.port' "$CONFIG_FILE" 2>/dev/null || echo "6221")
GRPC_PORT=$(yq '.schedules.port' "$CONFIG_FILE" 2>/dev/null || echo "50043")
WHATSAPP_PORT=$(yq '.whatsnyan.grpc_port' "$CONFIG_FILE" 2>/dev/null || echo "50042")
SCHEDULER_PORT=$(yq '.grpc.port' "$CONFIG_FILE" 2>/dev/null || echo "50041")
N8N_PORT=$(yq '.n8n.port' "$CONFIG_FILE" 2>/dev/null || echo "5775")

# Monitoring ports from config
PROMETHEUS_PORT=9090  # Prometheus service port (hardcoded in docker-compose)
API_METRICS_PORT=$(yq '.metrics.api_port' "$CONFIG_FILE" 2>/dev/null || echo "9095")
GRAFANA_PORT=$(yq '.metrics.grafana_port' "$CONFIG_FILE" 2>/dev/null || echo "3063")
LOKI_PORT=3100  # Not in config, hardcoded in docker-compose
TEMPO_PORT=3200  # Not in config, hardcoded in docker-compose
NGINX_PORT=9180  # Not in config, hardcoded in docker-compose
K6_PORT=$(yq '.k6.port' "$CONFIG_FILE" 2>/dev/null || echo "6680")

# Counters for summary
TOTAL_CHECKS=0
PASSED_CHECKS=0
PASSED_SERVICES=()
FAILED_SERVICES=()

# Function to print status
print_status() {
    local service=$1
    local status=$2
    local details=$3
    TOTAL_CHECKS=$((TOTAL_CHECKS + 1))
    
    if [ "$status" = "PASS" ]; then
        echo -e "${GREEN}✅ $service: $details${NC}"
        PASSED_CHECKS=$((PASSED_CHECKS + 1))
        PASSED_SERVICES+=("$service")
    elif [ "$status" = "WARN" ]; then
        echo -e "${YELLOW}⚠️  $service: $details${NC}"
        FAILED_SERVICES+=("$service (Warning)")
    else
        echo -e "${RED}❌ $service: $details${NC}"
        FAILED_SERVICES+=("$service")
    fi
}

# Function to check if port is open
check_port() {
    local host=$1
    local port=$2
    if nc -z -w5 "$host" "$port" 2>/dev/null; then
        return 0
    else
        return 1
    fi
}

# Function to check container status
check_container() {
    local container_name=$1
    if podman ps --format "{{.Names}}" | grep -q "^${container_name}$"; then
        return 0
    else
        return 1
    fi
}

# Function to check health endpoint
check_health_endpoint() {
    local url=$1
    local expected=$2
    if curl -s --max-time 10 "$url" | grep -q "$expected"; then
        return 0
    else
        return 1
    fi
}

# Determine container runtime
if command -v podman-compose &> /dev/null; then
    COMPOSE_CMD="podman-compose"
    RUNTIME="Podman"
elif command -v docker-compose &> /dev/null; then
    COMPOSE_CMD="docker-compose"
    RUNTIME="Docker"
else
    print_status "Container Runtime" "FAIL" "Neither podman-compose nor docker-compose found"
    exit 1
fi

print_status "Container Runtime" "PASS" "$RUNTIME detected"

echo ""
echo "🔍 Checking Service Containers/Processes..."
echo "=========================================="

# Check API service (port check)
if check_port "localhost" "$API_PORT"; then
    print_status "API Service" "PASS" "Port $API_PORT is open"
    # Try health endpoint if available
    if curl -s --max-time 5 "http://localhost:$API_PORT/health" > /dev/null 2>&1; then
        print_status "API Health Endpoint" "PASS" "/health endpoint responding"
    else
        print_status "API Health Endpoint" "WARN" "/health endpoint not available"
    fi
    # Check API metrics endpoint
    if check_port "localhost" "$API_METRICS_PORT"; then
        print_status "API Metrics Endpoint" "PASS" "Port $API_METRICS_PORT accessible"
    else
        print_status "API Metrics Endpoint" "WARN" "Port $API_METRICS_PORT not accessible (API not running)"
    fi
else
    print_status "API Service" "FAIL" "Port $API_PORT is not accessible"
fi

# Check gRPC service
if check_port "localhost" "$GRPC_PORT"; then
    print_status "gRPC Service" "PASS" "Port $GRPC_PORT is open"
else
    print_status "gRPC Service" "FAIL" "Port $GRPC_PORT is not accessible"
fi

# Check WhatsApp service
if check_port "localhost" "$WHATSAPP_PORT"; then
    print_status "WhatsApp Service" "PASS" "Port $WHATSAPP_PORT is open"
else
    print_status "WhatsApp Service" "FAIL" "Port $WHATSAPP_PORT is not accessible"
fi

# Check Scheduler service
if check_port "localhost" "$SCHEDULER_PORT"; then
    print_status "Scheduler Service" "PASS" "Port $SCHEDULER_PORT is open"
else
    print_status "Scheduler Service" "FAIL" "Port $SCHEDULER_PORT is not accessible"
fi

echo ""
echo "📊 Checking Monitoring Stack..."
echo "=============================="

# Check Prometheus
if check_container "service-platform-prometheus"; then
    print_status "Prometheus Container" "PASS" "Running"
    if check_port "localhost" "$PROMETHEUS_PORT"; then
        print_status "Prometheus Port" "PASS" "Port $PROMETHEUS_PORT accessible"
        if check_health_endpoint "http://localhost:$PROMETHEUS_PORT/-/healthy" "Prometheus"; then
            print_status "Prometheus Health" "PASS" "Health endpoint responding"
        else
            print_status "Prometheus Health" "WARN" "Health endpoint not responding"
        fi
    else
        print_status "Prometheus Port" "FAIL" "Port $PROMETHEUS_PORT not accessible"
    fi
else
    print_status "Prometheus Container" "FAIL" "Not running"
fi

# Check Grafana
if check_container "service-platform-grafana"; then
    print_status "Grafana Container" "PASS" "Running"
    if check_port "localhost" "$GRAFANA_PORT"; then
        print_status "Grafana Port" "PASS" "Port $GRAFANA_PORT accessible"
    else
        print_status "Grafana Port" "FAIL" "Port $GRAFANA_PORT not accessible"
    fi
else
    print_status "Grafana Container" "FAIL" "Not running"
fi

# Check Loki
if check_container "service-platform-loki"; then
    print_status "Loki Container" "PASS" "Running"
    if check_port "localhost" "$LOKI_PORT"; then
        print_status "Loki Port" "PASS" "Port $LOKI_PORT accessible"
        if check_health_endpoint "http://localhost:$LOKI_PORT/ready" "ready"; then
            print_status "Loki Health" "PASS" "Ready endpoint responding"
        else
            print_status "Loki Health" "WARN" "Ready endpoint not responding"
        fi
    else
        print_status "Loki Port" "FAIL" "Port $LOKI_PORT not accessible"
    fi
else
    print_status "Loki Container" "FAIL" "Not running"
fi

# Check Tempo
if check_container "service-platform-tempo"; then
    print_status "Tempo Container" "PASS" "Running"
    if check_port "localhost" "$TEMPO_PORT"; then
        print_status "Tempo Port" "PASS" "Port $TEMPO_PORT accessible"
    else
        print_status "Tempo Port" "FAIL" "Port $TEMPO_PORT not accessible"
    fi
else
    print_status "Tempo Container" "FAIL" "Not running"
fi

# Check Nginx
if check_container "service-platform-nginx-auth"; then
    print_status "Nginx Container" "PASS" "Running"
    if check_port "localhost" "$NGINX_PORT"; then
        print_status "Nginx Port" "PASS" "Port $NGINX_PORT accessible"
    else
        print_status "Nginx Port" "FAIL" "Port $NGINX_PORT not accessible"
    fi
else
    print_status "Nginx Container" "FAIL" "Not running"
fi

echo ""
echo "🤖 Checking Additional Services..."
echo "================================="

# Check n8n
if check_container "service-platform-n8n"; then
    print_status "n8n Container" "PASS" "Running"
    if check_port "localhost" "$N8N_PORT"; then
        print_status "n8n Port" "PASS" "Port $N8N_PORT accessible"
    else
        print_status "n8n Port" "FAIL" "Port $N8N_PORT not accessible"
    fi
else
    print_status "n8n Container" "FAIL" "Not running"
fi

# Check k6
if check_container "service-platform-k6"; then
    print_status "k6 Container" "PASS" "Running"
    if check_port "localhost" "$K6_PORT"; then
        print_status "k6 Port" "PASS" "Port $K6_PORT accessible"
    else
        print_status "k6 Port" "FAIL" "Port $K6N_PORT not accessible"
    fi
else
    print_status "k6 Container" "FAIL" "Not running"
fi

echo ""
echo "🗄️  Checking Database Connectivity..."
echo "==================================="

# Check MongoDB
MONGO_PORT=$(yq '.mongodb.port' "$CONFIG_FILE" 2>/dev/null || echo "27007")
MONGO_HOST=$(yq '.mongodb.host' "$CONFIG_FILE" 2>/dev/null || echo "localhost")

if check_container "service-platform-mongodb"; then
    print_status "MongoDB Container" "PASS" "Running"
    if check_port "$MONGO_HOST" "$MONGO_PORT"; then
        print_status "MongoDB Port" "PASS" "Port $MONGO_PORT accessible"
        # Try to connect using mongosh (if available in container)
        if podman exec service-platform-mongodb mongosh --quiet --eval "db.adminCommand('ping')" mongodb://mongo_admin:password_admin_mongo@localhost:27017/service_platform_mongo_test?authSource=admin 2>/dev/null | grep -q "ok.*1"; then
            print_status "MongoDB Connection" "PASS" "Database responding to ping"
        else
            print_status "MongoDB Connection" "WARN" "Ping command not verified (mongosh may not be available)"
        fi
    else
        print_status "MongoDB Port" "FAIL" "Port $MONGO_PORT not accessible"
    fi
else
    print_status "MongoDB Container" "WARN" "Not running (optional service)"
fi

# Check MongoExpress
MONGOEXPRESS_PORT=$(yq '.mongoexpress.port' "$CONFIG_FILE" 2>/dev/null || echo "8081")
if check_container "service-platform-mongoexpress"; then
    print_status "MongoExpress Container" "PASS" "Running"
    if check_port "localhost" "$MONGOEXPRESS_PORT"; then
        print_status "MongoExpress Port" "PASS" "Port $MONGOEXPRESS_PORT accessible"
    else
        print_status "MongoExpress Port" "FAIL" "Port $MONGOEXPRESS_PORT not accessible"
    fi
else
    print_status "MongoExpress Container" "WARN" "Not running (optional service)"
fi

# Try PostgreSQL first
if command -v psql &> /dev/null; then
    # Try to connect to PostgreSQL (using default dev config values)
    if PGPASSWORD="Takasitau" psql -h "127.0.0.1" -p "5432" -U "swi" -d "db_service_platform_dev_rm" -c "SELECT 1;" --quiet --no-align --tuples-only 2>/dev/null | grep -q "1"; then
        print_status "PostgreSQL Database" "PASS" "Connected successfully"
    else
        print_status "PostgreSQL Database" "FAIL" "Connection failed"
    fi
else
    print_status "PostgreSQL Client" "WARN" "psql not available, cannot check DB connectivity"
fi

# Try MySQL as fallback
if command -v mysql &> /dev/null; then
    if mysql -h "127.0.0.1" -P "3306" -u "root" -e "SELECT 1;" 2>/dev/null | grep -q "1"; then
        print_status "MySQL Database" "PASS" "Connected successfully"
    else
        print_status "MySQL Database" "WARN" "Connection failed or not configured"
    fi
else
    print_status "MySQL Client" "INFO" "mysql not available"
fi

echo ""
echo "📋 Health Check Summary"
echo "======================"
echo "Total checks: $TOTAL_CHECKS"
echo "Passed: $PASSED_CHECKS"
echo "Failed: $((TOTAL_CHECKS - PASSED_CHECKS))"

if [ ${#PASSED_SERVICES[@]} -gt 0 ]; then
    echo ""
    echo "✅ Services that PASSED:"
    for service in "${PASSED_SERVICES[@]}"; do
        echo "   • $service"
    done
fi

if [ ${#FAILED_SERVICES[@]} -gt 0 ]; then
    echo ""
    echo "❌ Services that FAILED:"
    for service in "${FAILED_SERVICES[@]}"; do
        echo "   • $service"
    done
fi

echo ""
if [ "$PASSED_CHECKS" -eq "$TOTAL_CHECKS" ]; then
    echo -e "${GREEN}🎉 All checks passed! System is healthy.${NC}"
elif [ "$PASSED_CHECKS" -ge "$((TOTAL_CHECKS * 3 / 4))" ]; then
    echo -e "${YELLOW}⚠️  Most services are healthy, but some issues detected.${NC}"
else
    echo -e "${RED}❌ Multiple services have issues. Check the output above.${NC}"
fi

echo ""
echo "ℹ️  Container Runtime: $RUNTIME"
echo "ℹ️  Monitoring Access:"
echo "   • Grafana: http://localhost:$GRAFANA_PORT"
echo "   • Prometheus: http://localhost:$PROMETHEUS_PORT"
echo "   • Loki: http://localhost:$LOKI_PORT"
echo "   • Tempo: http://localhost:$TEMPO_PORT"
echo "   • Grafana Nginx (auth): http://localhost:$NGINX_PORT"
echo ""
echo "ℹ️  Database Access:"
echo "   • MongoExpress: http://localhost:$MONGOEXPRESS_PORT (admin/pass)"
echo "   • MongoDB: mongodb://mongo_admin:password_admin_mongo@localhost:$MONGO_PORT"