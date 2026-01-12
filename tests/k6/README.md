# k6 Load Testing Scripts

This directory contains k6 load testing scripts for the Service Platform API.

## Overview

k6 is a modern load testing tool built for testing the performance of APIs, microservices, and websites. These scripts test various aspects of the Service Platform under different load conditions.

## Available Test Scripts

### 1. `health-check.js`
**Purpose**: Basic health check endpoint load test

- **Duration**: 2 minutes
- **Load**: Ramps 0 → 10 → 10 → 0 VUs
- **Target**: `/health` endpoint
- **Thresholds**:
  - 95% of requests < 500ms
  - Error rate < 1%

**Run**:
```bash
make k6-health-check
# or
k6 run tests/k6/health-check.js
```

### 2. `api-smoke-test.js`
**Purpose**: Quick validation of critical endpoints

- **Duration**: 1 minute
- **Load**: 1 VU (minimal load)
- **Targets**: Multiple public endpoints
- **Tests**:
  - Health check
  - Hello endpoint
  - Metrics endpoint
  - Swagger documentation
  - Rate limiting behavior

**Run**:
```bash
make k6-smoke-test
# or
k6 run tests/k6/api-smoke-test.js
```

### 3. `login-flow.js`
**Purpose**: Test authentication flow under load

- **Duration**: 3 minutes
- **Load**: Ramps to 5 VUs
- **Tests**:
  - Captcha generation
  - Login submission
  - Session handling
- **Note**: Some failures are expected due to rate limiting and captcha validation

**Run**:
```bash
make k6-login-test
# or
k6 run tests/k6/login-flow.js
```

### 4. `stress-test.js`
**Purpose**: Progressive load increase to find breaking points

- **Duration**: 14 minutes
- **Load Profile**:
  - 0-2min: Ramp to 20 VUs
  - 2-5min: Ramp to 50 VUs
  - 5-7min: **SPIKE to 100 VUs**
  - 7-10min: Hold at 100 VUs
  - 10-14min: Ramp down
- **Tests**: Multiple weighted endpoints
- **Thresholds**:
  - 95% of requests < 3s
  - Error rate < 10% (higher tolerance for stress)
  - Minimum 50 req/s throughput

**Run**:
```bash
make k6-stress-test
# or
k6 run tests/k6/stress-test.js
```

## Running Tests

### Option 1: Using Makefile (Recommended)
```bash
# Start monitoring stack (includes k6)
make monitoring-start

# Run specific test
make k6-health-check
make k6-smoke-test
make k6-login-test
make k6-stress-test

# Run custom script
make k6-run-script SCRIPT=your-test.js
```

### Option 2: Using k6 CLI Directly
```bash
# Run local k6
k6 run tests/k6/health-check.js

# With custom environment variables
API_BASE_URL=http://localhost:6221 k6 run tests/k6/api-smoke-test.js

# With specific VUs and duration
k6 run --vus 20 --duration 30s tests/k6/health-check.js
```

### Option 3: Using Container
```bash
# Using Podman Compose
podman-compose -f docker-compose.monitoring.yml run --rm k6 run /scripts/health-check.js

# Using Docker Compose
docker-compose -f docker-compose.monitoring.yml run --rm k6 run /scripts/health-check.js
```

## Environment Variables

Configure tests using environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `API_BASE_URL` | `http://host.containers.internal:6221` | API base URL |
| `GRPC_BASE_URL` | `host.containers.internal:50041` | gRPC service URL |
| `WHATSAPP_GRPC_URL` | `host.containers.internal:50042` | WhatsApp gRPC URL |
| `SCHEDULER_GRPC_URL` | `host.containers.internal:50043` | Scheduler gRPC URL |
| `TEST_EMAIL` | `wegirandol@smartwebindonesia.com` | Test account email |
| `TEST_PASSWORD` | `Net55206011##` | Test account password |

Example with custom environment:
```bash
API_BASE_URL=http://production-api.example.com k6 run tests/k6/health-check.js
```

## Viewing Results

### Console Output
k6 provides real-time CLI output during test execution showing:
- Current VUs, iterations, and duration
- Request metrics (rate, duration, failures)
- Check pass rates
- Custom metrics

### Web Dashboard
k6 includes a web dashboard available during test execution:
- URL: http://localhost:6668
- Shows real-time graphs and metrics
- Auto-exports HTML report to `/results/k6-report.html`

### Grafana Integration
All k6 metrics are automatically exported to Prometheus and visualized in Grafana:

1. **Access Grafana**: http://localhost:3063
   - Username: `admin`
   - Password: `Net55206011##`

2. **View k6 Dashboard**: Navigate to "k6 Load Testing Dashboard"

3. **Metrics Available**:
   - HTTP request duration (p50, p95, p99)
   - Request rate and throughput
   - Error rates
   - Virtual users over time
   - Custom metrics from scripts

### Prometheus Metrics
k6 exports metrics to Prometheus at: http://localhost:5665/metrics

Available metric types:
- `k6_http_reqs_total` - Total HTTP requests
- `k6_http_req_duration_seconds` - Request duration histogram
- `k6_http_req_failed_total` - Failed requests
- `k6_vus` - Active virtual users
- `k6_iterations_total` - Total iterations completed
- Custom metrics defined in scripts

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
    'http_req_failed': ['rate<0.01'],
  },
};

const API_BASE_URL = __ENV.API_BASE_URL || 'http://localhost:6221';

export default function () {
  const response = http.get(`${API_BASE_URL}/your-endpoint`);
  
  check(response, {
    'status is 200': (r) => r.status === 200,
    'response time OK': (r) => r.timings.duration < 500,
  });
  
  sleep(1);
}
```

### Best Practices
1. **Use groups** to organize test logic
2. **Add checks** to validate responses
3. **Use thresholds** to define success criteria
4. **Add think time** with `sleep()` to simulate real users
5. **Use custom metrics** for business-specific measurements
6. **Parameterize** with environment variables
7. **Add setup/teardown** functions for test lifecycle management

## Thresholds Configuration

Thresholds are pass/fail criteria for your tests. Common patterns:

```javascript
export const options = {
  thresholds: {
    // HTTP metrics
    'http_req_duration': ['p(95)<500', 'p(99)<1000'],  // Percentiles
    'http_req_failed': ['rate<0.01'],                  // Error rate
    'http_reqs': ['rate>100'],                         // Minimum throughput
    
    // Custom metrics
    'login_duration': ['p(95)<2000'],                  // Custom trend
    'errors': ['rate<0.05'],                           // Custom rate
    
    // By tag/group
    'http_req_duration{name:Login}': ['p(95)<1000'],   // Specific endpoint
  },
};
```

## CI/CD Integration

### Exit Codes
- `0`: Test passed (all thresholds met)
- `99`: Test failed (one or more thresholds breached)
- `Other`: Technical error

### Example CI Script
```bash
#!/bin/bash
set -e

# Start services
make monitoring-start

# Wait for services to be ready
sleep 10

# Run smoke test
make k6-smoke-test

# Run load test
make k6-health-check

# Cleanup
make monitoring-stop
```

## Troubleshooting

### Connection Refused
- Ensure API is running: `curl http://localhost:6221/health`
- Check if monitoring stack is running: `make monitoring-status`
- Verify ports are not blocked by firewall

### High Error Rates
- Check API logs: `tail -f log/app.log`
- Verify rate limiting settings in `config.dev.yaml`
- Reduce VUs or increase ramp-up time

### Metrics Not Showing in Grafana
- Verify Prometheus is scraping k6: http://localhost:9090/targets
- Check k6 Prometheus endpoint: http://localhost:5665/metrics
- Restart monitoring stack: `make monitoring-deep-restart`

### Container Issues
```bash
# Check k6 container status
podman ps -a | grep k6

# View k6 container logs
podman logs service-platform-k6

# Rebuild k6 container
podman-compose -f docker-compose.monitoring.yml up -d --force-recreate k6
```

## Resources

- [k6 Documentation](https://k6.io/docs/)
- [k6 Examples](https://k6.io/docs/examples/)
- [k6 Prometheus Output](https://k6.io/docs/results-output/real-time/prometheus-remote-write/)
- [k6 Best Practices](https://k6.io/docs/testing-guides/test-types/)

## Support

For issues or questions:
1. Check this README
2. Review k6 documentation
3. Check monitoring logs
4. Contact the platform team
