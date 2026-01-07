/**
 * k6 Load Test: Login Flow
 * 
 * Tests the complete login flow including:
 * 1. Getting captcha
 * 2. Submitting login credentials
 * 3. Session validation
 * 
 * Run: k6 run login-flow.js
 */

import http from 'k6/http';
import { check, group, sleep } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';
import { randomString } from 'https://jslib.k6.io/k6-utils/1.2.0/index.js';

// Custom metrics
const errorRate = new Rate('errors');
const loginAttempts = new Counter('login_attempts');
const successfulLogins = new Counter('successful_logins');
const failedLogins = new Counter('failed_logins');
const loginDuration = new Trend('login_duration');

// Test configuration
export const options = {
  stages: [
    { duration: '30s', target: 5 },   // Ramp up to 5 VUs
    { duration: '2m', target: 5 },    // Stay at 5 VUs
    { duration: '30s', target: 0 },   // Ramp down
  ],
  thresholds: {
    'http_req_duration': ['p(95)<2000'],     // 95% of requests below 2s
    'http_req_failed': ['rate<0.05'],        // Error rate below 5%
    'login_duration': ['p(95)<3000'],        // Login should complete in 3s
    'errors': ['rate<0.1'],                  // Custom error rate below 10%
  },
};

// Configuration
const API_BASE_URL = __ENV.API_BASE_URL || 'http://localhost:6221';
const TEST_EMAIL = __ENV.TEST_EMAIL || 'wegirandol@smartwebindonesia.com';
const TEST_PASSWORD = __ENV.TEST_PASSWORD || 'Ro224171222#';

export default function () {
  const startTime = new Date();
  let loginSuccess = false;

  group('Login Flow', function () {
    // Step 1: Get Login Page (optional, but simulates real user)
    group('Get Login Page', function () {
      const loginPageRes = http.get(`${API_BASE_URL}/login`);
      check(loginPageRes, {
        'login page status is 200': (r) => r.status === 200,
      });
    });

    // Step 2: Get Captcha
    let captchaId = '';
    group('Get Captcha', function () {
      const captchaRes = http.get(`${API_BASE_URL}/gen_captcha`);
      const captchaCheck = check(captchaRes, {
        'captcha status is 200': (r) => r.status === 200,
        'captcha has id': (r) => {
          try {
            const body = JSON.parse(r.body);
            captchaId = body.captcha_id || body.id || '';
            return captchaId.length > 0;
          } catch (e) {
            return false;
          }
        },
      });
      
      if (!captchaCheck) {
        console.warn('Captcha generation failed, using random ID');
        captchaId = randomString(32);
      }
    });

    // Step 3: Submit Login
    group('Submit Login', function () {
      const loginPayload = JSON.stringify({
        email: TEST_EMAIL,
        password: TEST_PASSWORD,
        captcha_id: captchaId,
        captcha_answer: '0000', // Placeholder - adjust based on your captcha implementation
      });

      const loginParams = {
        headers: {
          'Content-Type': 'application/json',
        },
        tags: { name: 'Login' },
      };

      const loginRes = http.post(`${API_BASE_URL}/login`, loginPayload, loginParams);
      
      loginAttempts.add(1);
      
      const loginCheck = check(loginRes, {
        'login status is 200 or 302': (r) => r.status === 200 || r.status === 302,
        'no error in response': (r) => !r.body.includes('error') && !r.body.includes('failed'),
        'login completes in time': (r) => r.timings.duration < 3000,
      });

      if (loginCheck) {
        successfulLogins.add(1);
        loginSuccess = true;
      } else {
        failedLogins.add(1);
        // Note: Some failures are expected due to rate limiting or captcha
        if (loginRes.status === 429) {
          console.log('Rate limited - this is expected behavior');
        } else if (loginRes.status === 400) {
          console.log('Bad request - likely captcha validation');
        }
      }
      
      errorRate.add(!loginCheck);
    });
  });

  // Record login duration
  const endTime = new Date();
  loginDuration.add(endTime - startTime);

  // Think time - simulate user reading content
  sleep(3 + Math.random() * 2); // 3-5 seconds
}

// Setup function
export function setup() {
  console.log('='.repeat(60));
  console.log('Login Flow Load Test');
  console.log('='.repeat(60));
  console.log(`Target: ${API_BASE_URL}`);
  console.log(`Test Account: ${TEST_EMAIL}`);
  console.log('⚠️  Note: Some login failures are expected due to:');
  console.log('   - Rate limiting');
  console.log('   - Captcha validation');
  console.log('   - Max retry login limits');
  console.log('='.repeat(60));
  
  // Verify API is reachable
  const healthCheck = http.get(`${API_BASE_URL}/health`);
  if (healthCheck.status !== 200) {
    throw new Error(`API is not reachable. Health check returned: ${healthCheck.status}`);
  }
  
  return { startTime: new Date() };
}

// Teardown function
export function teardown(data) {
  const endTime = new Date();
  const duration = (endTime - data.startTime) / 1000;
  
  console.log('='.repeat(60));
  console.log(`Login Flow Test Completed in ${duration.toFixed(2)}s`);
  console.log('='.repeat(60));
}
