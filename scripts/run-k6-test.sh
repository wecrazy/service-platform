#!/bin/bash

# k6 Test Runner Script
# This script runs k6 load tests using the containerized k6 service
# Usage: ./run-k6-test.sh <test-script.js>

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
K6_SCRIPTS_DIR="$PROJECT_ROOT/tests/k6"
COMPOSE_FILE="$PROJECT_ROOT/docker/docker-compose.monitoring.yml"

# Check if script argument is provided
if [ -z "$1" ]; then
    echo -e "${RED}❌ Error: No test script provided${NC}"
    echo ""
    echo "Usage: $0 <test-script.js>"
    echo ""
    echo "Available test scripts:"
    ls -1 "$K6_SCRIPTS_DIR"/*.js 2>/dev/null | xargs -n 1 basename || echo "  No test scripts found"
    exit 1
fi

TEST_SCRIPT="$1"

# Check if test script exists
if [ ! -f "$K6_SCRIPTS_DIR/$TEST_SCRIPT" ]; then
    echo -e "${RED}❌ Error: Test script not found: $K6_SCRIPTS_DIR/$TEST_SCRIPT${NC}"
    echo ""
    echo "Available test scripts:"
    ls -1 "$K6_SCRIPTS_DIR"/*.js 2>/dev/null | xargs -n 1 basename || echo "  No test scripts found"
    exit 1
fi

# Detect container runtime
detect_runtime() {
    if command -v podman-compose &> /dev/null; then
        echo "podman-compose"
    elif command -v docker-compose &> /dev/null; then
        echo "docker-compose"
    elif command -v docker &> /dev/null && docker compose version &> /dev/null; then
        echo "docker compose"
    else
        echo -e "${RED}❌ Error: Neither podman-compose nor docker-compose found${NC}"
        exit 1
    fi
}

RUNTIME=$(detect_runtime)

# Load configuration
CONFIG_MODE=$(yq -r '.config_mode' "$PROJECT_ROOT/internal/config/conf.yaml" 2>/dev/null || echo "dev")
CONFIG_FILE="$PROJECT_ROOT/internal/config/service-platform.${CONFIG_MODE}.yaml"
API_PORT=$(yq -r '.app.port' "$CONFIG_FILE" 2>/dev/null || echo "6221")

# Function to check if monitoring stack is running
check_monitoring() {
    if [ "$RUNTIME" = "podman-compose" ]; then
        podman ps --filter "name=service-platform-prometheus" --format "{{.Names}}" | grep -q "service-platform-prometheus"
    elif [ "$RUNTIME" = "docker compose" ]; then
        docker ps --filter "name=service-platform-prometheus" --format "{{.Names}}" | grep -q "service-platform-prometheus"
    else
        docker-compose -f "$COMPOSE_FILE" ps prometheus | grep -q "Up"
    fi
}

# Function to check if API is reachable
check_api() {
    local API_URL="http://localhost:${API_PORT}/health"
    if curl -s -f "$API_URL" > /dev/null 2>&1; then
        return 0
    else
        return 1
    fi
}

# Print banner
echo -e "${BLUE}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║${NC}          ${GREEN}k6 Load Testing - Service Platform${NC}                  ${BLUE}║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════════════╝${NC}"
echo ""

# Check monitoring stack
echo -e "${YELLOW}🔍 Checking monitoring stack...${NC}"
if ! check_monitoring; then
    echo -e "${YELLOW}⚠️  Monitoring stack not running. Starting...${NC}"
    "$SCRIPT_DIR/start-monitoring.sh"
    echo -e "${GREEN}✓${NC} Monitoring stack started"
    echo ""
    # Wait for services to be ready
    echo -e "${YELLOW}⏳ Waiting for services to be ready (10 seconds)...${NC}"
    sleep 10
else
    echo -e "${GREEN}✓${NC} Monitoring stack is running"
fi
echo ""

# Check API service
echo -e "${YELLOW}🔍 Checking API service...${NC}"
if ! check_api; then
    echo -e "${YELLOW}⚠️  Warning: API service may not be reachable at http://localhost:6221${NC}"
    echo -e "${YELLOW}   Test may fail if the API is not running.${NC}"
    echo -e "${YELLOW}   Start the API with: make run-api${NC}"
    echo ""
    read -p "Continue anyway? (y/N): " -n 1 -r
    echo ""
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo -e "${RED}❌ Test aborted${NC}"
        exit 1
    fi
else
    echo -e "${GREEN}✓${NC} API service is reachable"
fi
echo ""

# Function to stop idle k6 container if running
ensure_k6_stopped() {
    local CONTAINER_NAME="service-platform-k6"
    if [ "$RUNTIME" = "podman-compose" ]; then
        if podman ps --format "{{.Names}}" | grep -q "^${CONTAINER_NAME}$"; then
            echo -e "${YELLOW}⚠️  Stopping idle ${CONTAINER_NAME} to free port 6680...${NC}"
            podman stop "${CONTAINER_NAME}" >/dev/null 2>&1 || true
            podman rm "${CONTAINER_NAME}" >/dev/null 2>&1 || true
        fi
    else
        if docker ps --format "{{.Names}}" | grep -q "^${CONTAINER_NAME}$"; then
            echo -e "${YELLOW}⚠️  Stopping idle ${CONTAINER_NAME} to free port 6680...${NC}"
            docker stop "${CONTAINER_NAME}" >/dev/null 2>&1 || true
            docker rm "${CONTAINER_NAME}" >/dev/null 2>&1 || true
        fi
    fi
}

# Run the k6 test
echo -e "${GREEN}🚀 Running k6 test: ${TEST_SCRIPT}${NC}"
echo -e "${BLUE}════════════════════════════════════════════════════════════════${NC}"
echo ""

# Ensure idle k6 is stopped to avoid port conflict
ensure_k6_stopped

# Set environment variables for the test
export API_BASE_URL="http://host.containers.internal:6221"
export K6_PROMETHEUS_RW_SERVER_URL="http://prometheus:9090/api/v1/write"
export K6_WEB_DASHBOARD="true"
export K6_WEB_DASHBOARD_PORT="6680"
export K6_OUT="experimental-prometheus-rw"

# Run k6 test based on runtime
if [ "$RUNTIME" = "podman-compose" ]; then
    podman-compose -f "$COMPOSE_FILE" run --rm --service-ports --entrypoint k6 k6 run "/scripts/$TEST_SCRIPT"
    TEST_EXIT_CODE=$?
elif [ "$RUNTIME" = "docker compose" ]; then
    docker compose -f "$COMPOSE_FILE" run --rm --service-ports --entrypoint k6 k6 run "/scripts/$TEST_SCRIPT"
    TEST_EXIT_CODE=$?
else
    docker-compose -f "$COMPOSE_FILE" run --rm --service-ports --entrypoint k6 k6 run "/scripts/$TEST_SCRIPT"
    TEST_EXIT_CODE=$?
fi

echo ""
echo -e "${BLUE}════════════════════════════════════════════════════════════════${NC}"

# Check test result
if [ $TEST_EXIT_CODE -eq 0 ]; then
    echo -e "${GREEN}✅ Test completed successfully!${NC}"
else
    echo -e "${RED}❌ Test failed with exit code: $TEST_EXIT_CODE${NC}"
fi

echo ""
echo -e "${BLUE}📊 View Results:${NC}"
echo -e "   ${YELLOW}Web Dashboard:${NC}    http://localhost:6680"
echo -e "   ${YELLOW}Grafana:${NC}          http://localhost:3063 (admin / Net55206011##)"
echo -e "   ${YELLOW}Prometheus:${NC}       http://localhost:9090"
echo ""

exit $TEST_EXIT_CODE
