# Service Platform Monitoring Setup

This monitoring setup uses **Podman Compose** as the primary container runtime, with Docker Compose as fallback. Podman provides better security (no root daemon) and is the recommended approach.

## Resource Usage & Optimization

### Storage & Memory Consumption

**Prometheus:**
- ✅ **DOES store data** on disk (time-series database)
- **Retention:** 72 hours (optimized for low usage)
- **Memory:** Limited to 256MB max
- **Disk:** ~50-200MB for 72 hours of metrics
- **Scrape Interval:** 30 seconds (reduced from 15s)

**Grafana:**
- ✅ **Stores metadata** (dashboards, users) in SQLite
- ✅ **Does NOT store metrics** (that's Prometheus' job)
- **Memory:** Limited to 128MB max
- **Logging:** Reduced to `warn` level only

### Optimized for Low Resource Usage

```yaml
# Prometheus optimizations:
- scrape_interval: 30s (not 15s)
- retention.time: 72h (not 200h)
- memory limit: 256MB
- smaller block durations for efficient storage

# Grafana optimizations:
- log_level: warn (not info/debug)
- memory limit: 128MB
- analytics disabled
- update checks disabled
```

### Data Cleanup

Run periodic cleanup to reduce disk usage:

```bash
# Using Makefile (recommended)
make monitoring-cleanup

# Or run script directly
./scripts/cleanup-monitoring.sh

# Or add to cron for automatic cleanup
# 0 */6 * * * cd /path/to/service-platform && make monitoring-cleanup
```

### Expected Resource Usage

| Component | Memory | Disk | CPU |
|-----------|--------|------|-----|
| Prometheus | ~50-150MB | ~50-200MB | Low |
| Grafana | ~30-80MB | ~20-50MB | Low |
| **Total** | **~100-250MB** | **~100-300MB** | **Minimal** |

### Why Prometheus Stores Data

- **Time-series database** for historical metrics
- **Compression:** Efficiently stores data points
- **Querying:** Enables historical analysis and alerting
- **Retention:** Configurable cleanup (we set 72h)

### Configurable Settings

The following metrics configuration is already added to `internal/config/service-platform.dev.yaml`:

```yaml
metrics:
  api_port: 9095        # Prometheus port for API metrics
  grpc_port: 9092       # Metrics port for gRPC service
  scheduler_port: 9091  # Metrics port for scheduler service
  whatsapp_port: 9093   # Metrics port for WhatsApp service
  telegram_port: 9094   # Metrics port for Telegram service
  grafana_port: 3063    # Grafana port
  libretranslate_port: 5004  # LibreTranslate port
```

And the corresponding struct is defined in `internal/config/config.go`:

```go
Metrics struct {
    APIPort            int `yaml:"api_port"`
    GRPCPort           int `yaml:"grpc_port"`
    SchedulerPort      int `yaml:"scheduler_port"`
    WhatsAppPort       int `yaml:"whatsapp_port"`
    TelegramPort       int `yaml:"telegram_port"`
    GrafanaPort        int `yaml:"grafana_port"`
    LibreTranslatePort int `yaml:"libretranslate_port"`
} `yaml:"metrics"`
```

When you create `internal/config/service-platform.prod.yaml`, make sure to include the same `metrics` section with your production port values.

## Why Podman?

Podman is preferred over Docker for several reasons:
- **No root daemon**: Runs containers as regular user
- **Better security**: Uses user namespaces and SELinux by default
- **Docker compatibility**: `podman-compose` reads standard docker-compose.yml files
- **Systemd integration**: Can be managed as system services
- **Rootless containers**: Enhanced security posture

## Overview

The monitoring stack includes:
- **Prometheus**: Collects metrics from all services
- **Grafana**: Visualizes metrics and creates dashboards (with automatic provisioning)
- **Service Metrics**: Each service exposes Prometheus metrics on `/metrics` endpoint

## Grafana Dashboards

Dashboards are automatically provisioned when Grafana starts. The following dashboards are included:

### Pre-configured Dashboards

1. **Service Platform Overview** - High-level view of all services health and key metrics
2. **API Service Dashboard** - Detailed metrics for the API service
3. **gRPC Service Dashboard** - Detailed metrics for the gRPC service
4. **Scheduler Service Dashboard** - Detailed metrics for the scheduler service
5. **WhatsApp Service Dashboard** - Detailed metrics for the WhatsApp service
6. **LibreTranslate Service** - Detailed metrics for the LibreTranslate service
7. **N8N Service** - Detailed metrics for the N8N service

### Dashboard Features

Each service dashboard includes:
- Service health status (UP/DOWN)
- Request rates and throughput
- Response/request duration percentiles
- Memory usage trends
- Auto-refresh every 5 seconds

### Customizing Dashboards

The dashboards are stored in `monitoring/grafana/dashboards/` as JSON files. You can:
- Edit the JSON files directly
- Add new panels in Grafana UI (changes won't persist across restarts)
- Create new dashboard JSON files in the same directory

### Dashboard Provisioning Files

The automatic dashboard setup uses these configuration files:

```
monitoring/grafana/
├── provisioning/
│   ├── dashboards/
│   │   └── dashboards.yml          # Dashboard provider configuration
│   └── datasources/
│       └── grafana-datasource.yml  # Grafana datasource configuration
└── dashboards/
    ├── overview-dashboard.json     # Main overview dashboard
    ├── api-service-dashboard.json
    ├── grpc-service-dashboard.json
    ├── scheduler-service-dashboard.json
    ├── whatsapp-service-dashboard.json
    ├── telegram-service-dashboard.json
    ├── libretranslate-dashboard.json
    └── n8n-service-dashboard.json
```

These files are mounted into the Grafana container and automatically loaded on startup.

## Services and Ports

- **API Service**: Main web API on port 9095, metrics on same port
- **Auth gRPC Service**: gRPC on port 50041, metrics on port from config (default 9092)
- **Scheduler Service**: gRPC on port 50043, metrics on port from config (default 9091)
- **WhatsApp Service**: gRPC on port 50042, metrics on port from config (default 9093)
- **Telegram Service**: gRPC on port 50044, metrics on port from config (default 9094)
- **LibreTranslate Service**: metrics running on port from config (default 5004)
- **N8N Service**: metrics running on port from config (default 5775)

## Metrics Exposed

The following system metrics are collected every 30 seconds:
- CPU load (1, 5, 15 minute averages)
- Memory usage (used %, total, used bytes)
- Disk usage (used %, total, used bytes)
- Network I/O (bytes sent/received)

## Setup Instructions

> **Note:** The monitoring stack uses its own compose file (`docker/docker-compose.monitoring.yml`).
> For full-stack local dev (databases + all app services), see `docker/docker-compose.yml` and `make docker-up`.

### 1. Start Monitoring Stack

The scripts automatically detect and prioritize Podman over Docker:

**Recommended: Using Podman Compose**
```bash
./scripts/start-monitoring.sh
```

**Alternative: Direct Podman Compose**
```bash
podman-compose -f docker/docker-compose.monitoring.yml up -d
```

**Fallback: Docker Compose (if Podman unavailable)**
```bash
docker-compose -f docker/docker-compose.monitoring.yml up -d
```

### 2. Stop Monitoring Stack

**Using the script:**
```bash
./scripts/stop-monitoring.sh
```

**Direct commands:**
```bash
podman-compose -f docker/docker-compose.monitoring.yml down -v
# or
docker-compose -f docker/docker-compose.monitoring.yml down -v
```

## Access URLs

The monitoring services will be available on ports configured in your YAML files:

- **Prometheus**: `http://localhost:{metrics.api_port}`
- **Grafana**: `http://localhost:{metrics.grafana_port}`

Check the script output for exact URLs after startup.

### 2. Start Services

Start each service in separate terminals:

```bash
# API Service
go run cmd/api/main.go

# Auth gRPC Service
go run cmd/grpc/main.go

# Scheduler Service
go run cmd/scheduler/main.go

# WhatsApp Service
go run cmd/whatsapp/main.go

# Telegram Service
go run cmd/telegram/main.go
```

### 3. Access Grafana

1. Open http://localhost:3063
2. Login with admin user
3. **✅ Dashboards are automatically provisioned and ready to use!**

**Available Dashboards:**
- **Service Platform Overview** - Main dashboard showing health and key metrics for all services
- **API Service Dashboard** - Detailed metrics for the API service
- **gRPC Service Dashboard** - Detailed metrics for the gRPC service  
- **Scheduler Service Dashboard** - Detailed metrics for the scheduler service
- **WhatsApp Service Dashboard** - Detailed metrics for the WhatsApp service
- **Telegram Service Dashboard** - Detailed metrics for the Telegram service
- **LibreTranslate Service Dashboard** - Detailed metrics for the LibreTranslate service
- **N8N Service Dashboard** - Detailed metrics for the N8N service

**Dashboard Features:**
- Auto-refresh every 5 seconds
- Service health status indicators (UP/DOWN)
- Request rates, response times, and memory usage
- 1-hour default time range with live updates

## Accessing Metrics

- Prometheus UI: http://localhost:9090
- Grafana: http://localhost:3063
- Individual service metrics:
  - API: http://localhost:9095/api-metrics
  - Auth gRPC: http://localhost:9092/grpc-metrics
  - Scheduler: http://localhost:9091/scheduler-metrics
  - WhatsApp: http://localhost:9093/whatsapp-metrics
  - Telegram: http://localhost:9094/telegram-metrics
  - LibreTranslate: http://localhost:5004/metrics
  - N8N: http://localhost:5775/metrics

## Troubleshooting

- Ensure all services are running and accessible
- Check Prometheus targets page for scrape status
- Verify firewall allows the metric ports
- Check service logs for any metric collection errors