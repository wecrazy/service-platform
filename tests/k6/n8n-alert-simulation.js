import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  scenarios: {
    normal_traffic: {
      executor: 'constant-vus',
      vus: 1,
      duration: '5s',
      exec: 'sendNormalTraffic',
    },
    high_traffic_alert: {
      executor: 'per-vu-iterations',
      vus: 1,
      iterations: 1,
      startTime: '6s', // Wait for normal traffic to finish
      exec: 'sendHighTrafficAlert',
    },
  },
};

// Use host.containers.internal to reach the host machine where n8n is exposed on port 5775
const BASE_URL = 'http://host.containers.internal:5775'; 

export function sendNormalTraffic() {
  const url = `${BASE_URL}/webhook/alert`;
  const payload = JSON.stringify({
    status: 'ok',
    value: 20,
    timestamp: new Date().toISOString(),
  });

  const params = {
    headers: {
      'Content-Type': 'application/json',
    },
  };

  const res = http.post(url, payload, params);
  
  check(res, {
    'normal status is 200': (r) => r.status === 200,
  });
  sleep(1);
}

export function sendHighTrafficAlert() {
  const url = `${BASE_URL}/webhook/alert`;
  const payload = JSON.stringify({
    status: 'firing',
    value: 95, // Above 80 threshold
    timestamp: new Date().toISOString(),
    service: 'api-service',
    description: 'CPU usage exceeded threshold',
  });

  const params = {
    headers: {
      'Content-Type': 'application/json',
    },
  };

  const res = http.post(url, payload, params);

  console.log(`Sent High Traffic Alert! Status: ${res.status}`);

  check(res, {
    'alert status is 200': (r) => r.status === 200,
  });
}
