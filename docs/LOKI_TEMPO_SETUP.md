# Loki + Tempo + Grafana Integration Guide

This guide covers the integrated observability stack with centralized logging (Loki) and distributed tracing (Tempo) protected by Nginx authentication.

## Components

### 1. **Loki** (Log Aggregation)
- **Port**: 3100
- **Purpose**: Centralized log aggregation from all services
- **Configuration**: `monitoring/loki/loki-config.yml`
- **Retention**: 72 hours
- **Data Storage**: `/loki` volume in container

### 2. **Tempo** (Distributed Tracing)
- **OTLP gRPC Port**: 4317
- **HTTP API Port**: 3200
- **Purpose**: Distributed tracing for request flow across services
- **Configuration**: `monitoring/tempo/tempo-config.yml`
- **Data Storage**: `/var/tempo` volume in container

### 3. **Nginx Reverse Proxy** (Authentication)
- **Port**: 9180
- **Purpose**: Secure Grafana, Loki, and Tempo with basic HTTP authentication
- **Configuration**: `monitoring/nginx/nginx.conf`
- **Credentials**: `monitoring/nginx/.htpasswd`

### 4. **Grafana** (Visualization)
- **Port**: 3063 (direct) or 9180 (via Nginx)
- **Datasources**: 
  - Prometheus (metrics)
  - Loki (logs)
  - Tempo (traces)

## Starting the Stack

### Start monitoring services
```bash
docker-compose -f docker/docker-compose.monitoring.yml up -d
```

### Verify services are running
```bash
docker-compose -f docker/docker-compose.monitoring.yml ps
```

### Check logs
```bash
# Loki logs
docker-compose -f docker/docker-compose.monitoring.yml logs -f loki

# Tempo logs
docker-compose -f docker/docker-compose.monitoring.yml logs -f tempo

# Nginx logs
docker-compose -f docker/docker-compose.monitoring.yml logs -f nginx-auth
```

## Accessing Services

### Direct Access (No Authentication)
- **Grafana**: http://localhost:3063
- **Loki**: http://localhost:3100
- **Tempo**: http://localhost:3200
- **Prometheus**: http://localhost:9090

### Via Nginx (With Authentication)
- **Grafana**: http://localhost:9180
- **Loki**: http://localhost:9180
- **Tempo**: http://localhost:9180
- **Health Check**: http://localhost:9180/health

### Credentials
- **Username**: `admin`
- **Password**: `Net55206011##`

## Configuration

### Application Configuration
Add to `internal/config/service-platform.dev.yaml`:

```yaml
observability:
  loki:
    enabled: true
    url: "http://127.0.0.1:3100"
    batch_size: 100
    batch_timeout_ms: 5000
    labels:
      service: "service-platform"
      environment: "development"
      version: "0.0.0.1.2025.12.26"
      hostname: "localhost"

  tempo:
    enabled: true
    otlp_grpc_endpoint: "http://127.0.0.1:4317"
    otlp_http_endpoint: "http://127.0.0.1:3200"
    sample_rate: 1.0  # 100% in dev, 0.1 in prod
    max_export_batch_size: 512
    export_timeout_ms: 30000

  jaeger:
    enabled: false
    endpoint: "http://127.0.0.1:14250"
```

### Logger Initialization
The logger automatically:
1. Writes logs to local CSV files (local debugging)
2. Sends logs to Loki (centralized aggregation)
3. Supports all existing log levels and formats

```go
import "service-platform/pkg/logger"

// Initialize logger (includes Loki hook if enabled)
logger.InitLogrus()
```

## Nginx Authentication

### Default Credentials
- Username: `admin`
- Password: `admin`

### Change Credentials
Generate new `.htpasswd` file:
```bash
# Install apache2-utils first if needed
sudo apt-get install apache2-utils

# Generate new credentials
openssl passwd -apr1

# Edit monitoring/nginx/.htpasswd
nano monitoring/nginx/.htpasswd
```

### Nginx Configuration
The nginx.conf provides:
- Basic HTTP authentication for all upstream services
- Rate limiting (10 req/s for API, 20 req/s for Loki)
- Compression (gzip)
- Large file upload support (100MB)
- Reverse proxy with proper headers

## Grafana Datasource Integration

### Auto-Provisioned Datasources
1. **Prometheus** (default)
   - Metrics from all services
   - Scrape interval: 30s
   - Retention: 72h

2. **Loki**
   - Centralized logs
   - Max lines per query: 1000
   - Full-text search capability

3. **Tempo**
   - Distributed traces
   - Node graph visualization enabled
   - Trace-to-logs correlation via trace_id

### Dashboard Creation
Create dashboards linking:
- HTTP/gRPC metrics (Prometheus)
- Request logs (Loki)
- Request traces (Tempo)

Use trace ID for correlation:
```
trace_id = "${trace_id}"
```

## Logging

### Local File Logging (CSV Format)
- Location: `./log/` directory
- Format: CSV with timestamp, level, message, caller
- Rotation: 10MB per file, 5 backups, 1 day retention
- Example: `redis_setup.log`, `main.log`, `scheduler.log`

### Centralized Logging (Loki)
- All logs forwarded to Loki automatically
- Label-based filtering by service, environment, version
- Queryable via LogQL in Grafana
- Retention: 72 hours

### Query Examples in Grafana

#### Find all error logs
```
{service="service-platform"} | "ERROR"
```

#### Find logs from specific service
```
{service="service-platform", environment="development"} | json
```

#### Metric: Log count per service
```
sum(rate({service="service-platform"}[5m])) by (service)
```

## Distributed Tracing

### Current Status
Tracing support is configured but instrumentation pending. Once services are instrumented:

### Trace Spans Include
- HTTP requests (method, path, status)
- gRPC calls (service, method)
- Database queries (query time, rows)
- Redis operations (command, latency)
- Service-to-service calls (latency)

### Tempo Queries
```
# Find traces with high latency
{ http.status_code=500 }

# Find traces from specific service
{ service.name="api-gateway" }

# Correlate to logs
Trace ID: click "View Logs" to see related logs in Loki
```

## Performance Tuning

### Loki
- Batch size: 100 logs per batch
- Batch timeout: 5 seconds
- Max lines: 1000 per Grafana query
- Retention: 72 hours (configurable)

### Tempo
- Sample rate: 100% (development), 10-30% (production)
- Max batch size: 512 spans
- Export timeout: 30 seconds
- Storage backend: local filesystem

### Nginx
- Worker connections: 1024
- Keep-alive timeout: 65 seconds
- Gzip compression: enabled
- Rate limits:
  - API endpoints: 10 req/s
  - Loki: 20 req/s

## Troubleshooting

### Services Not Starting
```bash
# Check docker-compose syntax
docker-compose -f docker/docker-compose.monitoring.yml config

# View service logs
docker-compose -f docker/docker-compose.monitoring.yml logs <service-name>
```

### Loki Connection Issues
```bash
# Test Loki connectivity
curl -v http://localhost:3100/api/v1/status

# Check Loki logs
docker-compose -f docker/docker-compose.monitoring.yml logs loki
```

### Tempo Connection Issues
```bash
# Test Tempo connectivity
curl -v http://localhost:3200/tempo/api/search

# Check Tempo logs
docker-compose -f docker/docker-compose.monitoring.yml logs tempo
```

### Nginx Authentication Issues
```bash
# Verify .htpasswd format
cat monitoring/nginx/.htpasswd

# Test with curl
curl -u admin:'Net55206011##' http://localhost:9180/health
```

## Next Steps

1. **Instrument Services**: Add OpenTelemetry SDK to all 7 services
2. **Create Dashboards**: Build Grafana dashboards for:
   - Service health overview
   - Request latency (P50, P95, P99)
   - Error rates
   - Log volume by service
3. **Alerts**: Set up Prometheus alert rules based on logs/traces
4. **Retention Policies**: Adjust based on usage and storage
5. **SSL/TLS**: Add SSL certificates to Nginx for production

## References

- [Grafana Loki Documentation](https://grafana.com/docs/loki/)
- [Grafana Tempo Documentation](https://grafana.com/docs/tempo/)
- [OpenTelemetry Go SDK](https://pkg.go.dev/go.opentelemetry.io/otel)
- [Logrus Documentation](https://github.com/sirupsen/logrus)
