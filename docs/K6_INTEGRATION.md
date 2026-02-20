# k6 Load Testing Integration

## Summary

Successfully integrated k6 load testing into Service Platform with Grafana visualization and Prometheus metrics collection.

## Changes Made

- **config.go**: Added `K6` struct with settings for enabled, ports, scripts directory, thresholds, and scenarios
- **internal/config/service-platform.dev.yaml**: Added k6 configuration section with:
  - Ports: 6668 (API/UI), 5665 (Prometheus)
  - Default thresholds for HTTP performance
  - 3 pre-configured test scenarios (health check, smoke test, stress test)

### 2. Docker/Container Setup
- **docker/docker-compose.monitoring.yml**: Added k6 service with:
  - Grafana k6 latest image
  - Prometheus remote write output
  - Volume mount for test scripts
  - Environment variables for service URLs
  - Host network access via `host.containers.internal`
  - Memory limits (512M)

### 3. Prometheus Integration
- **prometheus.yml**: Added k6 job configuration
  - Scrapes k6 metrics from port 5665
  - 15s scrape interval for real-time metrics

### 4. Test Scripts (tests/k6/)
Created 4 comprehensive test scripts:
- **health-check.js**: Basic health endpoint load test (10 VUs, 2min)
- **api-smoke-test.js**: Quick validation of critical endpoints (1 VU, 1min)
- **login-flow.js**: Authentication flow test (5 VUs, 3min)
- **stress-test.js**: Progressive load test (0→100 VUs, 14min)
- **README.md**: Complete documentation for k6 testing

### 5. Grafana Dashboard
- **monitoring/grafana/dashboards/k6-load-testing.json**: 
  - HTTP request duration (p50, p95, p99)
  - Virtual users over time
  - Request rate and throughput
  - Error rates with alerts
  - Stats by endpoint table
  - Data sent/received graphs
  - Summary statistics panels

### 6. Makefile Targets
Added k6 commands:
- `make k6-health-check` - Run health check test
- `make k6-smoke-test` - Run smoke test
- `make k6-login-test` - Run login test
- `make k6-stress-test` - Run stress test
- `make k6-run-script SCRIPT=test.js` - Run custom test
- `make k6-status` - Check k6 status
- `make k6-stop` - Stop tests
- `make k6-results` - View results

### 7. Scripts
- **scripts/run-k6-test.sh**: Automated test runner with:
  - Container runtime detection (Podman/Docker)
  - Monitoring stack verification
  - API service health check
  - Colored output and error handling

## Quick Start

### 1. Start Monitoring Stack
```bash
make monitoring-start
```

### 2. Start API Service
```bash
make run-api
```

### 3. Run Load Tests
```bash
# Quick smoke test (1 minute)
make k6-smoke-test

# Health check load test (2 minutes)
make k6-health-check

# Login flow test (3 minutes)
make k6-login-test

# Full stress test (14 minutes)
make k6-stress-test
```

### 4. View Results
- **Web Dashboard**: http://localhost:6668
- **Grafana**: http://localhost:3063 (admin / Net55206011##)
- **Prometheus**: http://localhost:9090

## Architecture

```
┌─────────────┐
│   k6 Test   │
│   Scripts   │
└──────┬──────┘
       │
       ├─────────────────┐
       │                 │
       ▼                 ▼
┌─────────────┐   ┌─────────────┐
│ k6 Engine   │──▶│ API Service │
│ (Container) │   │  :6221      │
└──────┬──────┘   └─────────────┘
       │
       │ Prometheus Remote Write
       │
       ▼
┌─────────────┐
│ Prometheus  │
│   :9090     │
└──────┬──────┘
       │
       │ Scrape & Store
       │
       ▼
┌─────────────┐
│   Grafana   │
│   :3063     │
└─────────────┘
```

## Features

### Test Capabilities
✅ HTTP endpoint testing (GET, POST)  
✅ Authentication flow testing  
✅ Rate limiting validation  
✅ Performance threshold checking  
✅ Custom metrics tracking  
✅ Weighted endpoint selection  
✅ Progressive load ramping  

### Monitoring Integration
✅ Real-time metrics in Prometheus  
✅ Pre-built Grafana dashboard  
✅ Web-based k6 dashboard  
✅ HTML report export  
✅ Alert thresholds  
✅ Historical data retention (72h)  

### CI/CD Ready
✅ Exit codes for pass/fail  
✅ Environment variable configuration  
✅ Container-based execution  
✅ Automated service verification  
✅ Script validation  

## Configuration

### Environment Variables
Tests can be configured via environment variables:
- `API_BASE_URL` - API service URL
- `TEST_EMAIL` - Test account email
- `TEST_PASSWORD` - Test account password
- `GRPC_BASE_URL` - gRPC service URL
- `K6_PROMETHEUS_RW_SERVER_URL` - Prometheus endpoint

### Thresholds (internal/config/service-platform.dev.yaml)
```yaml
thresholds:
  http_req_duration: "p(95)<500"      # 95% under 500ms
  http_req_failed: "rate<0.01"        # <1% error rate
  http_reqs_per_second: 10            # Min 10 req/s
  iteration_duration: "p(95)<2000"    # 95% under 2s
  checks_pass_rate: "rate>0.95"       # 95% pass rate
```

### Test Scenarios
Pre-configured scenarios in internal/config/service-platform.dev.yaml:
1. **health-check-load-test** - Constant 10 VUs for 30s
2. **api-smoke-test** - Single VU for 1m
3. **stress-test** - Ramping VUs up to 100 for 5m

## Port Allocation

| Service | Port | Purpose |
|---------|------|---------|
| k6 Web UI | 6668 | Dashboard and API |
| k6 Prometheus | 5665 | Metrics endpoint |
| API Service | 6221 | Target application |
| Grafana | 3063 | Visualization |
| Prometheus | 9090 | Metrics storage |

## Writing Custom Tests

### Basic Template
```javascript
import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  vus: 10,
  duration: '30s',
  thresholds: {
    'http_req_duration': ['p(95)<500'],
  },
};

const API_BASE_URL = __ENV.API_BASE_URL || 'http://localhost:6221';

export default function () {
  const response = http.get(`${API_BASE_URL}/your-endpoint`);
  check(response, {
    'status is 200': (r) => r.status === 200,
  });
  sleep(1);
}
```

Save to `tests/k6/your-test.js` and run:
```bash
make k6-run-script SCRIPT=your-test.js
```

## Troubleshooting

### Issue: Connection Refused
**Solution**: Ensure API is running
```bash
curl http://localhost:6221/health
make run-api
```

### Issue: Metrics Not in Grafana
**Solution**: Restart monitoring stack
```bash
make monitoring-deep-restart
```

### Issue: High Error Rates
**Solution**: Check API logs and rate limits
```bash
tail -f log/app.log
# Adjust rate_limit settings in internal/config/service-platform.dev.yaml
```

## Next Steps

1. **Add gRPC Tests**: Create tests for gRPC services (auth, whatsapp, scheduler)
2. **Authenticated Endpoints**: Expand tests for protected routes
3. **WebSocket Tests**: Add WebSocket connection testing
4. **CI/CD Integration**: Add to GitHub Actions or GitLab CI
5. **Performance Baselines**: Establish performance benchmarks
6. **Alert Rules**: Configure Grafana alerts for test failures

## Resources

- [k6 Documentation](https://k6.io/docs/)
- [k6 Prometheus Integration](https://k6.io/docs/results-output/real-time/prometheus-remote-write/)
- [Test Scripts README](tests/k6/README.md)
- [Grafana k6 Plugin](https://grafana.com/grafana/plugins/grafana-k6-app/)

## Support

For questions or issues:
1. Check [tests/k6/README.md](tests/k6/README.md)
2. Review k6 logs: `podman logs service-platform-k6`
3. Check monitoring status: `make monitoring-status`
4. Review test output in terminal
