/**
 * k6 Smoke Test: Critical API Endpoints
 * 
 * Validates that critical endpoints are working correctly with minimal load.
 * This is a quick sanity check before running full load tests.
 * 
 * Run: k6 run api-smoke-test.js
 */

import http from 'k6/http';
import { check, group, sleep } from 'k6';
import { Rate, Counter } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');
const endpointChecks = new Counter('endpoint_checks');

// Test configuration - low load for smoke testing
export const options = {
  vus: 1,           // Single virtual user
  duration: '1m',   // Run for 1 minute
  thresholds: {
    'http_req_duration': ['p(95)<1000'],   // 95% of requests should be below 1s
    'http_req_failed': ['rate<0.05'],      // Error rate should be below 5%
    'checks': ['rate>0.95'],               // 95% of checks should pass
  },
};

// Configuration from environment variables
const API_BASE_URL = __ENV.API_BASE_URL || 'http://localhost:6221';

export default function () {
  // Group: Public Endpoints
  group('Public Endpoints', function () {
    // Test 1: Health endpoint
    group('Health Check', function () {
      const healthRes = http.get(`${API_BASE_URL}/health`);
      const healthCheck = check(healthRes, {
        'health status is 200': (r) => r.status === 200,
        'health response time < 500ms': (r) => r.timings.duration < 500,
      });
      endpointChecks.add(1);
      errorRate.add(!healthCheck);
    });

    // Test 2: Hello endpoint
    group('Hello Endpoint', function () {
      const helloRes = http.get(`${API_BASE_URL}/hello`);
      const helloCheck = check(helloRes, {
        'hello status is 200': (r) => r.status === 200,
        'hello has response body': (r) => r.body.length > 0,
      });
      endpointChecks.add(1);
      errorRate.add(!helloCheck);
    });

    // Test 3: Metrics endpoint
    group('Metrics Endpoint', function () {
      const metricsRes = http.get(`${API_BASE_URL}/api-metrics`);
      const metricsCheck = check(metricsRes, {
        'metrics status is 200': (r) => r.status === 200,
        'metrics content type is text/plain': (r) => r.headers['Content-Type']?.includes('text/plain'),
      });
      endpointChecks.add(1);
      errorRate.add(!metricsCheck);
    });

    // Test 4: Swagger documentation
    group('Swagger Documentation', function () {
      const swaggerRes = http.get(`${API_BASE_URL}/swagger`);
      const swaggerCheck = check(swaggerRes, {
        'swagger is accessible': (r) => r.status === 200 || r.status === 301 || r.status === 302,
      });
      endpointChecks.add(1);
      errorRate.add(!swaggerCheck);
    });
  });

  // Group: Rate Limit Check
  group('Rate Limiting', function () {
    const responses = [];
    
    // Make multiple rapid requests to test rate limiting
    for (let i = 0; i < 5; i++) {
      responses.push(http.get(`${API_BASE_URL}/health`));
    }
    
    const rateLimitCheck = check(responses, {
      'all requests completed': (r) => r.length === 5,
      'rate limit working or all passed': (r) => {
        const allSuccess = r.every(res => res.status === 200);
        const someRateLimited = r.some(res => res.status === 429);
        return allSuccess || someRateLimited;
      },
    });
    
    endpointChecks.add(1);
    errorRate.add(!rateLimitCheck);
  });

  // Think time between iterations
  sleep(2);
}

// Setup function
export function setup() {
  console.log('='.repeat(60));
  console.log('API Smoke Test');
  console.log('='.repeat(60));
  console.log(`Target: ${API_BASE_URL}`);
  console.log(`VUs: 1`);
  console.log(`Duration: 1 minute`);
  console.log('='.repeat(60));
  
  // Verify API is reachable
  const healthCheck = http.get(`${API_BASE_URL}/health`);
  if (healthCheck.status !== 200) {
    console.error(`⚠️  WARNING: API health check failed with status ${healthCheck.status}`);
    console.error('Test will continue but may fail...');
  } else {
    console.log('✓ API is reachable');
  }
  
  return { startTime: new Date() };
}

// Teardown function
export function teardown(data) {
  const endTime = new Date();
  const duration = (endTime - data.startTime) / 1000;
  
  console.log('='.repeat(60));
  console.log(`API Smoke Test Completed in ${duration.toFixed(2)}s`);
  console.log('='.repeat(60));
}
