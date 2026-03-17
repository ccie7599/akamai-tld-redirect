// k6 Load Test — TLD Redirect Engine
//
// Tests the data plane redirect engine under load with 2000 domains.
// Seeds domains via the control plane API, then hammers the data plane.
//
// Usage:
//   # Seed 2000 domains first (run once):
//   k6 run --vus 1 --iterations 1 -e PHASE=seed -e CONTROL_URL=https://tld-control-ord.connected-cloud.io -e TOKEN=<token> k6-load-test.js
//
//   # Run load test against data plane:
//   k6 run -e PHASE=load -e DATA_URL=http://172.238.165.172 k6-load-test.js
//
//   # Full test (seed + load):
//   k6 run -e PHASE=full -e CONTROL_URL=https://tld-control-ord.connected-cloud.io -e TOKEN=<token> -e DATA_URL=http://172.238.165.172 k6-load-test.js

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

const redirectLatency = new Trend('redirect_latency', true);
const redirectSuccess = new Rate('redirect_success');

const PHASE = __ENV.PHASE || 'load';
const CONTROL_URL = __ENV.CONTROL_URL || 'https://tld-control-ord.connected-cloud.io';
const DATA_URL = __ENV.DATA_URL || 'http://172.238.165.172';
const TOKEN = __ENV.TOKEN || '';
const DOMAIN_COUNT = parseInt(__ENV.DOMAIN_COUNT || '2000');

// Load test options
export const options = PHASE === 'seed' ? {
  vus: 1,
  iterations: 1,
} : {
  stages: [
    { duration: '10s', target: 50 },   // ramp up
    { duration: '60s', target: 200 },   // sustain
    { duration: '30s', target: 500 },   // peak
    { duration: '10s', target: 0 },     // ramp down
  ],
  thresholds: {
    'redirect_latency': ['p(95)<50', 'p(99)<100'],  // 95th < 50ms, 99th < 100ms
    'redirect_success': ['rate>0.99'],                // 99%+ success
    'http_req_duration': ['p(95)<100'],
  },
};

// Generate domain list
function generateDomains(count) {
  const domains = [];
  const prefixes = ['acme', 'legacy', 'old', 'acquired', 'merged', 'former', 'heritage', 'classic', 'vintage', 'retro'];
  const types = ['bank', 'financial', 'savings', 'trust', 'credit', 'capital', 'mutual', 'national', 'community', 'regional'];
  const tlds = ['example.com', 'example.net', 'example.org'];

  for (let i = 0; i < count; i++) {
    const prefix = prefixes[i % prefixes.length];
    const type = types[Math.floor(i / prefixes.length) % types.length];
    const tld = tlds[Math.floor(i / (prefixes.length * types.length)) % tlds.length];
    domains.push(`${prefix}-${type}-${i}.${tld}`);
  }
  return domains;
}

const DOMAINS = generateDomains(DOMAIN_COUNT);

// Seed phase — bulk import domains via control plane API
function seedDomains() {
  const batchSize = 200;
  let imported = 0;

  for (let i = 0; i < DOMAINS.length; i += batchSize) {
    const batch = DOMAINS.slice(i, i + batchSize).map(d => ({
      domain: d,
      default_url: 'https://www.consolidated-bank.example.com',
      status_code: 301,
      rules: [
        { path: '/mortgage', target_url: 'https://www.consolidated-bank.example.com/mortgage', status_code: 301, priority: 10 },
        { path: '/about', target_url: 'https://www.consolidated-bank.example.com/about', status_code: 301, priority: 5 },
      ],
    }));

    const res = http.post(
      `${CONTROL_URL}/api/v1/import?token=${TOKEN}`,
      JSON.stringify(batch),
      { headers: { 'Content-Type': 'application/json' }, timeout: '30s' }
    );

    check(res, {
      'seed batch imported': (r) => r.status === 200,
    });

    if (res.status === 200) {
      const body = JSON.parse(res.body);
      imported += body.imported || 0;
    } else {
      console.error(`Seed batch failed: ${res.status} ${res.body}`);
    }
  }

  console.log(`Seeded ${imported} domains`);

  // Verify domain count
  const verify = http.get(`${CONTROL_URL}/api/v1/domains?token=${TOKEN}&limit=1`);
  if (verify.status === 200) {
    const body = JSON.parse(verify.body);
    console.log(`Total domains in DB: ${body.total}`);
  }
}

// Load phase — hit data plane with redirect requests
function loadTest() {
  const domain = DOMAINS[Math.floor(Math.random() * DOMAINS.length)];
  const paths = ['/', '/mortgage', '/about', '/contact', '/nonexistent'];
  const path = paths[Math.floor(Math.random() * paths.length)];

  const res = http.get(`${DATA_URL}${path}`, {
    headers: { 'Host': domain },
    redirects: 0,  // Don't follow redirects — we want the 301
    timeout: '5s',
  });

  const success = res.status === 301 || res.status === 302;
  redirectSuccess.add(success);
  redirectLatency.add(res.timings.duration);

  check(res, {
    'is redirect': (r) => r.status === 301 || r.status === 302,
    'has location header': (r) => r.headers['Location'] !== undefined,
    'latency < 50ms': (r) => r.timings.duration < 50,
  });
}

export default function () {
  if (PHASE === 'seed') {
    seedDomains();
  } else if (PHASE === 'load') {
    loadTest();
  } else if (PHASE === 'full') {
    if (__ITER === 0 && __VU === 1) {
      seedDomains();
      sleep(5); // Wait for engine reload
    }
    loadTest();
  }
}

export function handleSummary(data) {
  const p95 = data.metrics.redirect_latency ? data.metrics.redirect_latency.values['p(95)'] : 'N/A';
  const p99 = data.metrics.redirect_latency ? data.metrics.redirect_latency.values['p(99)'] : 'N/A';
  const rate = data.metrics.redirect_success ? data.metrics.redirect_success.values.rate : 'N/A';
  const reqs = data.metrics.http_reqs ? data.metrics.http_reqs.values.count : 0;
  const rps = data.metrics.http_reqs ? data.metrics.http_reqs.values.rate : 0;

  console.log(`\n=== Load Test Results ===`);
  console.log(`Total requests:    ${reqs}`);
  console.log(`Requests/sec:      ${rps.toFixed(1)}`);
  console.log(`Success rate:      ${(rate * 100).toFixed(2)}%`);
  console.log(`Latency p95:       ${p95.toFixed(2)}ms`);
  console.log(`Latency p99:       ${p99.toFixed(2)}ms`);
  console.log(`Domains tested:    ${DOMAIN_COUNT}`);

  return {
    'stdout': '',
    'tests/load/results.json': JSON.stringify(data, null, 2),
  };
}
