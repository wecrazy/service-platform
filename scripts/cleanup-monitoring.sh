#!/bin/bash

# Service Platform Monitoring Cleanup Script
# Reduces disk usage by cleaning old Prometheus data

set -e

echo "🧹 Cleaning up Service Platform monitoring data..."

# Function to get directory size
get_dir_size() {
    local dir=$1
    if [ -d "$dir" ]; then
        du -sh "$dir" 2>/dev/null | cut -f1
    else
        echo "N/A"
    fi
}

# Function to show cleanup summary
show_cleanup_summary() {
    local before=$1
    local after=$2
    local item=$3

    if [ "$before" != "N/A" ] && [ "$after" != "N/A" ]; then
        echo "📊 $item: $before → $after"
    else
        echo "📊 $item: $before → $after"
    fi
}

# Check if monitoring is running and get initial sizes
echo "📈 Gathering initial monitoring data sizes..."

PROMETHEUS_SIZE_BEFORE=$(get_dir_size "./monitoring/prometheus")
echo "   📁 Prometheus data size: $PROMETHEUS_SIZE_BEFORE"

GRAFANA_SIZE_BEFORE="N/A"

if podman ps --filter "name=service-platform-grafana" --format "{{.Names}}" | grep -q "service-platform-grafana"; then
    echo "📈 Grafana is running, checking log size..."
    GRAFANA_SIZE_BEFORE=$(podman exec service-platform-grafana du -sh /var/log/grafana 2>/dev/null | cut -f1 || echo "N/A")
    echo "   📁 Grafana logs size: $GRAFANA_SIZE_BEFORE"
fi

echo ""
echo "🧽 Starting cleanup process..."

# Clean Prometheus data
if podman ps --filter "name=service-platform-prometheus" --format "{{.Names}}" | grep -q "service-platform-prometheus"; then
    echo "📊 Cleaning Prometheus data inside container..."

    # Count files before cleanup
    PROMETHEUS_FILES_BEFORE=$(podman exec service-platform-prometheus find /prometheus -type f | wc -l 2>/dev/null || echo "0")

    # Clean old Prometheus data (older than 2 hours)
    podman exec service-platform-prometheus find /prometheus -name "*.db" -type f -mmin +120 -delete 2>/dev/null || true
    podman exec service-platform-prometheus find /prometheus -name "chunks_head" -type d -mmin +120 -exec rm -rf {} + 2>/dev/null || true

    # Get size after cleanup
    PROMETHEUS_SIZE_AFTER=$(podman exec service-platform-prometheus du -sh /prometheus 2>/dev/null | cut -f1 || echo "N/A")
    PROMETHEUS_FILES_AFTER=$(podman exec service-platform-prometheus find /prometheus -type f | wc -l 2>/dev/null || echo "0")

    echo "✅ Prometheus cleanup completed"
    echo "   📁 Files: $PROMETHEUS_FILES_BEFORE → $PROMETHEUS_FILES_AFTER"
else
    echo "ℹ️  Prometheus is not running, cleaning host volume..."
    
    if [ -d "./monitoring/prometheus" ]; then
        # Count files before cleanup
        PROMETHEUS_FILES_BEFORE=$(find ./monitoring/prometheus -type f 2>/dev/null | wc -l || echo "0")

        # Clean old Prometheus data on host (older than 2 hours)
        find ./monitoring/prometheus -name "*.db" -type f -mmin +120 -delete 2>/dev/null || true
        find ./monitoring/prometheus -name "chunks_head" -type d -mmin +120 -exec rm -rf {} + 2>/dev/null || true

        # Get size after cleanup
        PROMETHEUS_SIZE_AFTER=$(get_dir_size "./monitoring/prometheus")
        PROMETHEUS_FILES_AFTER=$(find ./monitoring/prometheus -type f 2>/dev/null | wc -l || echo "0")

        echo "✅ Prometheus host cleanup completed"
        echo "   📁 Files: $PROMETHEUS_FILES_BEFORE → $PROMETHEUS_FILES_AFTER"
    else
        echo "ℹ️  Prometheus data directory does not exist, skipping host cleanup"
        PROMETHEUS_SIZE_AFTER="N/A"
        PROMETHEUS_FILES_AFTER="0"
    fi
fi

# Clean Grafana logs
if podman ps --filter "name=service-platform-grafana" --format "{{.Names}}" | grep -q "service-platform-grafana"; then
    echo "📈 Cleaning Grafana logs..."

    # Get log file size before cleanup
    GRAFANA_LOG_SIZE_BEFORE=$(podman exec service-platform-grafana du -sh /var/log/grafana/grafana.log 2>/dev/null | cut -f1 || echo "N/A")
    GRAFANA_LOG_LINES_BEFORE=$(podman exec service-platform-grafana wc -l /var/log/grafana/grafana.log 2>/dev/null | awk '{print $1}' || echo "0")

    # Remove old Grafana logs (keep only last 100 lines)
    podman exec service-platform-grafana sh -c 'if [ -f /var/log/grafana/grafana.log ]; then tail -n 100 /var/log/grafana/grafana.log > /tmp/grafana.log.tmp && mv /tmp/grafana.log.tmp /var/log/grafana/grafana.log; fi' 2>/dev/null || true

    # Get size after cleanup
    GRAFANA_SIZE_AFTER=$(podman exec service-platform-grafana du -sh /var/log/grafana 2>/dev/null | cut -f1 || echo "N/A")
    GRAFANA_LOG_SIZE_AFTER=$(podman exec service-platform-grafana du -sh /var/log/grafana/grafana.log 2>/dev/null | cut -f1 || echo "N/A")
    GRAFANA_LOG_LINES_AFTER=$(podman exec service-platform-grafana wc -l /var/log/grafana/grafana.log 2>/dev/null | awk '{print $1}' || echo "0")

    echo "✅ Grafana logs cleanup completed"
    echo "   📄 Log lines: $GRAFANA_LOG_LINES_BEFORE → $GRAFANA_LOG_LINES_AFTER"
else
    echo "ℹ️  Grafana is not running, skipping cleanup"
fi

echo ""
echo "📊 Cleanup Summary:"
echo "=================="

if [ "$PROMETHEUS_SIZE_BEFORE" != "N/A" ]; then
    show_cleanup_summary "$PROMETHEUS_SIZE_BEFORE" "$PROMETHEUS_SIZE_AFTER" "Prometheus data"
fi

if [ "$GRAFANA_SIZE_BEFORE" != "N/A" ]; then
    show_cleanup_summary "$GRAFANA_SIZE_BEFORE" "$GRAFANA_SIZE_AFTER" "Grafana logs"
fi

echo ""
echo "🎉 Monitoring cleanup finished!"