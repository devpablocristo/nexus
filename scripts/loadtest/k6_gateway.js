import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '30s', target: 10 },
    { duration: '1m', target: 50 },
    { duration: '30s', target: 100 },
    { duration: '30s', target: 0 },
  ],
  thresholds: {
    http_req_duration: ['p(95)<500'],
    http_req_failed: ['rate<0.01'],
  },
};

export default function () {
  const url = `${__ENV.API_URL || 'http://localhost:8080'}/v1/run`;
  const payload = JSON.stringify({
    tool_name: 'echo',
    input: { message: `k6-${__ITER}` },
  });
  const params = {
    headers: {
      'Content-Type': 'application/json',
      'X-NEXUS-CORE-KEY': __ENV.API_KEY || 'dev-api-key',
    },
  };

  const res = http.post(url, payload, params);
  check(res, {
    'status is 200': (r) => r.status === 200,
    'latency < 500ms': (r) => r.timings.duration < 500,
  });
  sleep(0.1);
}
