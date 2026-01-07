/**
 * k6 Stress Test: High Load Scenario
 * 
 * Progressively increases load to find breaking points and test system resilience.
 * Tests multiple endpoints under increasing pressure.
 * 
 * Run: k6 run stress-test.js
 */

import http from 'k6/http';
import { check, group, sleep } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');
const requestCounter = new Counter('total_requests');
const successCounter = new Counter('successful_requests');
const failureCounter = new Counter('failed_requests');
const responseTime = new Trend('custom_response_time');

// Test configuration - stress test with ramping load
export const options = {
  stages: [
    { duration: '2m', target: 20 },    // Ramp up to 20 users
    { duration: '3m', target: 50 },    // Ramp up to 50 users
    { duration: '2m', target: 100 },   // Spike to 100 users
    { duration: '3m', target: 100 },   // Stay at 100 users
    { duration: '2m', target: 50 },    // Ramp down to 50
    { duration: '2m', target: 0 },     // Ramp down to 0
  ],
  thresholds: {
    'http_req_duration': ['p(95)<3000'],      // 95% under 3s even under stress
    'http_req_failed': ['rate<0.1'],          // Allow 10% errors under stress
    'http_reqs': ['rate>50'],                 // Maintain at least 50 req/s
    'errors': ['rate<0.15'],                  // Custom error rate below 15%
  },
};

// Configuration
const API_BASE_URL = __ENV.API_BASE_URL || 'http://localhost:6221';

// Endpoint list for stress testing
const endpoints = [
  { name: 'health', url: '/health', method: 'GET', weight: 50 },
  { name: 'hello', url: '/hello', method: 'GET', weight: 30 },
  { name: 'metrics', url: '/api-metrics', method: 'GET', weight: 10 },
  { name: 'swagger', url: '/swagger', method: 'GET', weight: 10 },
];

// Weighted random endpoint selector
function selectEndpoint() {
  const totalWeight = endpoints.reduce((sum, ep) => sum + ep.weight, 0);
  let random = Math.random() * totalWeight;
  
  for (const endpoint of endpoints) {
    random -= endpoint.weight;
    if (random <= 0) {
      return endpoint;
    }
  }
  
  return endpoints[0]; // Fallback
}

export default function () {
  // Select endpoint based on weight
  const endpoint = selectEndpoint();
  
  const url = `${API_BASE_URL}${endpoint.url}`;
  const params = {
    headers: {
      'Content-Type': 'application/json',
      'User-Agent': 'k6-stress-test',
    },
    tags: { 
      name: endpoint.name,
      endpoint: endpoint.url,
    },
  };

  requestCounter.add(1);
  
  let response;
  if (endpoint.method === 'GET') {
    response = http.get(url, params);
  } else {
    response = http.post(url, null, params);
  }

  responseTime.add(response.timings.duration);

  // Check response
  const checkResult = check(response, {
    'status is successful': (r) => r.status >= 200 && r.status < 400,
    'response has body': (r) => r.body && r.body.length > 0,
    'response time acceptable': (r) => r.timings.duration < 5000,
  });

  if (checkResult) {
    successCounter.add(1);
  } else {
    failureCounter.add(1);
    
    // Log errors during high load
    if (response.status >= 500) {
      console.error(`Server error ${response.status} on ${endpoint.name}: ${response.body.substring(0, 100)}`);
    } else if (response.status === 429) {
      // Rate limiting is expected under stress
      console.log(`Rate limited on ${endpoint.name} (expected under stress)`);
    }
  }

  errorRate.add(!checkResult);

  // Variable think time based on load
  const currentVUs = __VU;
  const thinkTime = currentVUs > 50 ? 0.1 : (currentVUs > 20 ? 0.5 : 1);
  sleep(thinkTime);
}

// Setup function
export function setup() {
  console.log('='.repeat(70));
  console.log('STRESS TEST - Progressive Load Increase');
  console.log('='.repeat(70));
  console.log(`Target: ${API_BASE_URL}`);
  console.log('Load Profile:');
  console.log('  0-2min:  Ramp to 20 VUs');
  console.log('  2-5min:  Ramp to 50 VUs');
  console.log('  5-7min:  SPIKE to 100 VUs');
  console.log('  7-10min: Hold at 100 VUs');
  console.log('  10-12min: Ramp down to 50 VUs');
  console.log('  12-14min: Ramp down to 0 VUs');
  console.log('='.repeat(70));
  console.log('Endpoints being tested (weighted):');
  endpoints.forEach(ep => {
    console.log(`  - ${ep.name.padEnd(15)} ${ep.weight}% ${ep.method} ${ep.url}`);
  });
  console.log('='.repeat(70));
  
  // Verify API is reachable
  const healthCheck = http.get(`${API_BASE_URL}/health`);
  if (healthCheck.status !== 200) {
    throw new Error(`API is not reachable. Health check returned: ${healthCheck.status}`);
  }
  
  console.log('✓ API is reachable - Starting stress test...\n');
  
  return { 
    startTime: new Date(),
    baselineLatency: healthCheck.timings.duration,
  };
}

// Teardown function
export function teardown(data) {
  const endTime = new Date();
  const duration = (endTime - data.startTime) / 1000;
  
  console.log('\n' + '='.repeat(70));
  console.log(`Stress Test Completed in ${(duration / 60).toFixed(2)} minutes`);
  console.log(`Baseline latency: ${data.baselineLatency.toFixed(2)}ms`);
  console.log('='.repeat(70));
  console.log('Check Grafana dashboards for detailed metrics analysis');
  console.log('='.repeat(70));
  
  // Final health check
  const finalHealthCheck = http.get(`${API_BASE_URL}/health`);
  if (finalHealthCheck.status === 200) {
    console.log('✓ API is still healthy after stress test');
    console.log(`  Final latency: ${finalHealthCheck.timings.duration.toFixed(2)}ms`);
  } else {
    console.error(`⚠️  API health check failed after stress test: ${finalHealthCheck.status}`);
  }
}
