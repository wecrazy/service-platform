# Loki & Tempo Quick Start

## 🚀 Quick Start (30 seconds)

```bash
# 1. Start monitoring stack
bash scripts/monitoring-quickstart.sh start

# 2. Run your application (logs automatically go to Loki)
go run ./cmd/api/main.go

# 3. View logs in Grafana
# Open: http://localhost:3063
# Explore → Loki → {service="service-platform"}
```

## 📊 Access Points

| Service | Direct | Via Nginx | Credentials |
|---------|--------|-----------|-------------|
| Grafana | http://localhost:3063 | http://localhost:9180 | admin/Net55206011## |
| Prometheus | http://localhost:9090 | - | None |
| Loki API | http://localhost:3100 | http://localhost:9180/loki | admin/Net55206011## |
| Tempo API | http://localhost:3200 | http://localhost:9180/tempo | admin/Net55206011## |

## ✅ Verify Setup

```bash
# Check all services running
bash scripts/monitoring-quickstart.sh status

# Test Loki
bash scripts/monitoring-quickstart.sh test-loki
# Response: {"state":"ok"}

# Test Tempo
bash scripts/monitoring-quickstart.sh test-tempo

# Test Nginx
bash scripts/monitoring-quickstart.sh test-nginx
```

## 🔄 How It Works

Your logs flow through both local files AND Loki:

```
Your App
  ↓
logrus.Info("message")
  ↓
├─→ Local CSV files (./log/*.log) - unchanged
└─→ Loki HTTP Push (async)
    ├─ Batch: 100 logs or every 5 seconds
    ├─ Endpoint: http://loki:3100/loki/api/v1/push
    └─ Labels: service, environment, version, hostname
         ↓
    Grafana Dashboard (searchable, filterable)
```

## 🛠️ Common Commands

```bash
# Start/Stop
bash scripts/monitoring-quickstart.sh start
bash scripts/monitoring-quickstart.sh stop
bash scripts/monitoring-quickstart.sh restart

# Logs & Status
bash scripts/monitoring-quickstart.sh logs          # all services
bash scripts/monitoring-quickstart.sh logs loki     # specific service
bash scripts/monitoring-quickstart.sh status

# Maintenance
bash scripts/monitoring-quickstart.sh rebuild       # rebuild images
bash scripts/monitoring-quickstart.sh clean         # remove volumes & containers
bash scripts/monitoring-quickstart.sh change-password  # update Nginx auth

# Testing
bash scripts/monitoring-quickstart.sh test-loki
bash scripts/monitoring-quickstart.sh test-tempo
bash scripts/monitoring-quickstart.sh test-nginx
```

## 📝 Log Queries in Grafana

After opening Grafana (http://localhost:3063):
1. Click **Explore** (left sidebar)
2. Select **Loki** as data source
3. Try these queries:

```promql
# All logs from your service
{service="service-platform"}

# Error logs only
{service="service-platform"} | "ERROR"

# Logs containing specific text
{service="service-platform"} | "database"

# JSON parsing (if your logs are JSON)
{service="service-platform"} | json | level="error"

# Time filter
{service="service-platform"} | timerange > 5m
```

## 🔐 Security

**Current Setup (Development):**
- Nginx basic auth: `admin` / `admin`
- HTTP only (no HTTPS)
- Isolated container network

**For Production:**
```bash
# Change default password
bash scripts/monitoring-quickstart.sh change-password

# Add HTTPS to monitoring/nginx/nginx.conf
# Configure secrets in docker-compose
# Change NGINX_PORT environment variable if needed (default: 9180)
```

## ⚙️ Configuration

Located in `internal/config/service-platform.dev.yaml`:

```yaml
observability:
  loki:
    enabled: true                      # ← Enable/disable Loki
    url: "http://127.0.0.1:3100"      # Loki endpoint
    batch_size: 100                    # Logs per batch
    batch_timeout_ms: 5000             # 5 second timeout
    labels:
      service: "service-platform"
      environment: "development"
      version: "0.0.0.1.2025.12.26"
      hostname: "localhost"

  tempo:
    enabled: true                      # ← Enable/disable Tempo
    otlp_grpc_endpoint: "http://127.0.0.1:4317"
    sample_rate: 1.0                   # 100% sampling (dev)
```

To disable Loki (keep local files only):
```yaml
observability:
  loki:
    enabled: false
```

## 🐛 Troubleshooting

**Loki not receiving logs?**
```bash
# Check Loki is running
bash scripts/monitoring-quickstart.sh test-loki

# View Loki logs
bash scripts/monitoring-quickstart.sh logs loki

# Verify app is running and logging
# Check: ./log/*.log files should be updating
```

**Can't access Grafana?**
```bash
# Direct access (no auth)
curl http://localhost:3063

# Via Nginx (with auth)
curl -u admin:'Net55206011##' http://localhost:9180
```

**Logs not appearing in Grafana?**
1. Wait 5-10 seconds for first batch to send
2. Query: `{service="service-platform"}`
3. Check time range in Grafana (top right)
4. Verify app is generating logs: `tail -f ./log/*.log`

## 📚 Full Documentation

For comprehensive details, see:
- [LOKI_TEMPO_SETUP.md](LOKI_TEMPO_SETUP.md) - Complete architecture & setup guide
- [Makefile](../Makefile) - All available commands

## 🚀 Next Steps

### Phase 2 (Optional) - Service Instrumentation
Trace individual request flows across services:
- Instrument API service with OpenTelemetry
- Add gRPC tracing
- Trace database queries
- View traces in Grafana Tempo

This requires changes to service code but infrastructure is already ready!

## ❓ Help

```bash
bash scripts/monitoring-quickstart.sh help
```

For more info:
- Visit: http://localhost:3063 (Grafana)
- Check logs: `bash scripts/monitoring-quickstart.sh logs`
- Read setup guide: `docs/LOKI_TEMPO_SETUP.md`
