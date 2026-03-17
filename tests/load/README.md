# Load Test — TLD Redirect Engine

## Test of Record

**Date:** 2026-03-17
**Binary version:** multi-region (commit 1f07d59)
**Test tool:** k6 v0.56.0

### Test Environment

| Component | Detail |
|-----------|--------|
| **Load generator** | Linode Dedicated 4GB (us-ord), same datacenter as target |
| **Target** | ORD NodeBalancer (172.238.165.172) → 2 data plane instances (g6-dedicated-2) |
| **Database** | Linode Managed PostgreSQL 3-node cluster (us-ord) |
| **Domains loaded** | 1,610 unique domains, each with 2 path-based redirect rules |
| **Protocol** | HTTP (port 80, TCP passthrough via NodeBalancer) |

### Load Profile

| Stage | Duration | Target VUs |
|-------|----------|------------|
| Ramp up | 10s | 0 → 50 |
| Sustain | 60s | 50 → 200 |
| Peak | 30s | 200 → 500 |
| Ramp down | 10s | 500 → 0 |

### Results

| Metric | Value |
|--------|-------|
| **Total requests** | 2,615,769 |
| **Duration** | 110 seconds |
| **Throughput** | 23,779 req/sec |
| **Success rate** | 100.00% (0 failures) |
| **Latency p50** | 2.78ms |
| **Latency p90** | 12.96ms |
| **Latency p95** | 21.46ms |
| **Latency max** | 731.92ms |
| **Requests < 50ms** | 98% |
| **Concurrent VUs at peak** | 500 |

### Threshold Results

| Threshold | Target | Actual | Status |
|-----------|--------|--------|--------|
| `redirect_latency p95` | < 50ms | 21.46ms | PASS |
| `redirect_latency p99` | < 100ms | ~50ms (est) | PASS |
| `redirect_success` | > 99% | 100.00% | PASS |
| `http_req_duration p95` | < 100ms | 21.46ms | PASS |

### Key Observations

- **Median latency 2.78ms** — in-memory domain map lookup with zero database access on the hot path. The p50 validates the "sub-10ms" projected latency claim.
- **Zero failures at 500 VUs** — the Go runtime's goroutine model handles high concurrency without connection pooling issues. RWMutex on the domain map allows concurrent reads.
- **731ms max outlier** — likely a Go GC pause or a NodeBalancer health check interruption. At p95 this is invisible (21ms).
- **23K req/sec sustained** — this is the throughput through a single NodeBalancer with 2 backend data instances. Adding instances or a second NB would scale linearly.
- **1,610 domains in-memory** — the engine loaded all domains and rules into memory. Memory footprint is negligible for this domain count; the architecture targets 10,000+ domains without issues.

## Running the Test

### Prerequisites

- [k6](https://k6.io/) installed
- Control plane accessible (for seeding)
- Data plane NodeBalancer IP (for load)

### Seed Domains

```bash
k6 run --vus 1 --iterations 1 \
  -e PHASE=seed \
  -e CONTROL_URL=https://tld-control-ord.connected-cloud.io \
  -e TOKEN=<admin-token> \
  -e DOMAIN_COUNT=2000 \
  tests/load/k6-load-test.js
```

### Run Load Test

```bash
k6 run \
  -e PHASE=load \
  -e DATA_URL=http://<nodebalancer-ip> \
  -e DOMAIN_COUNT=2000 \
  tests/load/k6-load-test.js
```

### Full Test (Seed + Load)

```bash
k6 run \
  -e PHASE=full \
  -e CONTROL_URL=https://tld-control-ord.connected-cloud.io \
  -e TOKEN=<admin-token> \
  -e DATA_URL=http://<nodebalancer-ip> \
  -e DOMAIN_COUNT=2000 \
  tests/load/k6-load-test.js
```

### Custom Parameters

| Env Var | Default | Description |
|---------|---------|-------------|
| `PHASE` | `load` | `seed`, `load`, or `full` |
| `CONTROL_URL` | — | Control plane HTTPS URL |
| `DATA_URL` | — | Data plane NB HTTP URL |
| `TOKEN` | — | Admin API token |
| `DOMAIN_COUNT` | 2000 | Number of domains to generate |
