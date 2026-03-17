# TLD Redirect Engine

Multi-region, high-availability redirect engine for managing thousands of top-level domain (TLD) redirects. Built for enterprises consolidating legacy domain portfolios after mergers and acquisitions — replacing F5 appliances (single point of failure) with a distributed, observable, API-driven architecture.

## Architecture

<p align="center">
  <a href="docs/diagrams/architecture.excalidraw">
    <img src="docs/diagrams/architecture.svg" alt="Multi-region architecture" width="900">
  </a>
</p>

**Single binary, `-mode` flag:**
- `-mode control` — Admin API, UI, cert provisioning (Let's Encrypt HTTP-01), ObjSt sync publisher
- `-mode data` — Redirect serving, DS2 beacon telemetry, analytics pipeline, cert reader
- No flag — legacy single-instance mode (SQLite, both planes combined)

## Request Flow

<p align="center">
  <a href="docs/diagrams/request-flow.excalidraw">
    <img src="docs/diagrams/request-flow.svg" alt="Request flow" width="850">
  </a>
</p>

## Quick Start (Local Dev)

```bash
# Build (with SQLite support for local dev)
make build

# Run with sample data
./bin/tld-redirect -seed sample-data/redirects.json -token dev-token

# Admin UI:  http://localhost:8080/ui/?token=dev-token
# Redirects: curl -sI -H "Host: old-brand-financial.example.com" http://localhost:8081/
```

## Production Deployment

```bash
# Build for production (pure Go, no CGO — PostgreSQL only)
make build-pg

# Deploy control plane
make deploy-control SERVER=<ip> ENV=path/to/env

# Deploy data plane
make deploy-data SERVER=<ip> ENV=path/to/env
```

See [docs/runbook.md](docs/runbook.md) for full operational procedures.

## Key Features

| Feature | Description |
|---------|-------------|
| **Multi-region** | 2 regions, 2 data nodes per region, managed PG per region |
| **Control/data separation** | Control plane failure doesn't impact redirect serving |
| **Cross-region sync** | Object Storage (S3-compatible) replicates rules across regions in <10s |
| **On-demand TLS** | CertMagic provisions Let's Encrypt certs for each domain automatically |
| **DS2 beacon telemetry** | Every redirect fires a beacon to Akamai DataStream 2 for edge-level observability |
| **Analytics pipeline** | Async batch writes, hourly rollups, top paths/referers, inactive domain detection |
| **API + UI** | Full REST API with OpenAPI spec; embedded SPA for browsing and management |
| **Priority path matching** | Rules sorted by priority; first match wins. Supports exact and prefix matching |

## Observability: DS2 Beacon Pattern

Every redirect fires a non-blocking beacon to Akamai DataStream 2 — encoding domain, status, path, target, client IP, user agent, and referer in the URL path. DS2 captures the full request path and delivers batched JSON to a webhook endpoint every 30 seconds.

<p align="center">
  <a href="docs/diagrams/ds2-beacon.excalidraw">
    <img src="docs/diagrams/ds2-beacon.svg" alt="DS2 beacon pipeline" width="850">
  </a>
</p>

See [docs/ds2-beacon.md](docs/ds2-beacon.md) for the full design — path encoding format, Akamai property configuration, dataset fields, Python decoder, and operational metrics.

## Control/Data Plane Separation

<p align="center">
  <a href="docs/diagrams/control-data-split.excalidraw">
    <img src="docs/diagrams/control-data-split.svg" alt="Control vs data plane" width="850">
  </a>
</p>

## Security: Akamai Integration Options

See [docs/akamai-integration.md](docs/akamai-integration.md) for notes on using Akamai API Gateway and App & API Protector in hybrid mode to add WAF/bot protection to the redirect infrastructure without moving serving logic to the edge.

## Cost

| Resource | Spec | Monthly |
|----------|------|---------|
| PG DBaaS x 2 | 3-node Nanode per region | $74 |
| Data compute x 4 | Dedicated 4GB | $260 |
| Control compute x 2 | Dedicated 4GB | $130 |
| NodeBalancer x 2 | 1 per region | $20 |
| Object Storage | 1 bucket | $5 |
| **Total** | | **~$489/mo** |

## Project Structure

```
cmd/tld-redirect/          Entry point, -mode flag, startup branching
internal/
  redirect/engine.go        In-memory domain map, priority-based path matching
  store/store.go            Dual SQLite/PG store, BulkImportReplace, analytics
  analytics/pipeline.go     Async batch writer + 5-min rollup worker
  beacon/beacon.go          DS2 fire-and-forget HTTP beacon (4 workers)
  certs/certs.go            CertMagic provisioner (control) + loader (data)
  certs/pg_storage.go       certmagic.Storage backed by PG advisory locks
  sync/sync.go              S3 publish/poll with ETag-based change detection
  api/handlers.go           REST API with syncer integration
  api/middleware.go          Token auth, CORS, request logging
  server/mux.go             Host-based request routing
  ui/handler.go             Embedded SPA static assets
web/static/                 Admin UI (vanilla JS SPA)
terraform/
  modules/region/           PG, NB, instances, firewalls per region
  modules/dns/              Akamai Edge DNS A records
  environments/prod/        2-region production wiring
scripts/                    systemd units, deploy scripts
sample-data/                10 sample legacy domains for demo
docs/
  ds2-beacon.md             DS2 beacon pattern (detailed)
  akamai-integration.md     WAF/API Gateway integration options
  runbook.md                Operational procedures
  diagrams/                 Excalidraw source + SVG exports
```
