/**
 * k6 Load Test: Health Check Endpoint
 * 
 * This test validates the /health endpoint under load.
 * Tests basic availability and response time of the health check.
 * 
 * Run: k6 run health-check.js
 * Or via container: podman-compose -f docker-compose.monitoring.yml run k6 run /scripts/health-check.js
 */

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');
const healthCheckDuration = new Trend('health_check_duration');
const successfulChecks = new Counter('successful_health_checks');

// Test configuration
export const options = {
  stages: [
    { duration: '30s', target: 10 },  // Ramp up to 10 VUs
    { duration: '1m', target: 10 },   // Stay at 10 VUs
    { duration: '30s', target: 0 },   // Ramp down to 0
  ],
  thresholds: {
    'http_req_duration': ['p(95)<500'],        // 95% of requests should be below 500ms
    'http_req_failed': ['rate<0.01'],          // Error rate should be below 1%
    'errors': ['rate<0.05'],                   // Custom error rate below 5%
    'health_check_duration': ['p(99)<1000'],   // 99% below 1 second
  },
};

// Configuration from environment variables
const API_BASE_URL = __ENV.API_BASE_URL || 'http://localhost:6221';

export default function () {
  const url = `${API_BASE_URL}/health`;
  
  const params = {
    headers: {
      'Content-Type': 'application/json',
    },
    tags: { name: 'HealthCheck' },
  };

  const response = http.get(url, params);
  
  // Track custom metrics
  healthCheckDuration.add(response.timings.duration);
  
  // Validate response
  const checkResult = check(response, {
    'status is 200': (r) => r.status === 200,
    'response has body': (r) => r.body.length > 0,
    'response time < 500ms': (r) => r.timings.duration < 500,
    'response is JSON': (r) => r.headers['Content-Type']?.includes('application/json'),
  });
  
  // Track errors
  errorRate.add(!checkResult);
  
  if (checkResult) {
    successfulChecks.add(1);
  }
  
  // Think time
  sleep(1);
}

// Setup function - runs once before the test
export function setup() {
  console.log(`Starting health check load test against: ${API_BASE_URL}`);
  console.log(`Test will run with ramping VUs: 0 -> 10 -> 10 -> 0`);
}

// Teardown function - runs once after the test
export function teardown(data) {
  console.log('Health check load test completed');
}
