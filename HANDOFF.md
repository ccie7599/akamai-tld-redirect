# Delivery Team Handoff

## What This Is

A multi-region redirect engine that replaces F5 appliances for managing 2000+ legacy domain redirects. Two regions (us-ord, us-iad), separated control/data planes, managed PostgreSQL per region, cross-region sync via Object Storage, and per-redirect telemetry via Akamai DataStream 2.

## What's Deployed

- 6 Linode instances (2 control, 4 data) running a single Go binary in different modes
- 2 Managed PostgreSQL clusters (3-node each, auto-failover)
- 2 NodeBalancers (TCP passthrough to data plane)
- 1 Object Storage bucket for cross-region rule sync
- 3 Akamai Edge DNS A records
- 1 Akamai edge property for DS2 beacon telemetry

## What the Customer Needs to Do

1. **Delegate DNS** — Point their apex domains (A records) to the NodeBalancer IPs, or delegate NS to Akamai Edge DNS
2. **Populate domains** — Import their full domain list via the bulk import API
3. **Validate redirects** — Test each domain resolves and redirects correctly
4. **Set up DS2 downstream** — Configure their ClickHouse/Splunk/S3 to receive DS2 webhook data
5. **Harden** — Replace the shared admin token with per-user auth if needed; restrict admin CIDRs to their networks

## Key Endpoints

| Endpoint | Purpose |
|----------|---------|
| `https://tld-control-ord.connected-cloud.io/ui/` | Admin UI (ORD) |
| `https://tld-control-iad.connected-cloud.io/ui/` | Admin UI (IAD) |
| `https://tld-control-{region}.connected-cloud.io/api/v1/` | REST API |
| NodeBalancer IPs (see runbook) | Redirect serving |

## Configuration

All configuration is via environment variables in `/opt/tld-redirect/env` on each instance. See `scripts/tld-redirect-control.service` and `scripts/tld-redirect-data.service` for the full flag set.

## What's Not Done

- **HashiCorp Vault for cert storage** — Current implementation stores TLS certs (including private keys) in PostgreSQL. This is functional but POC-grade for secret management. For production, implement the Vault storage adapter (see ADR-005 in DECISIONS.md). Most enterprise customers will require this.
- **Production cert provisioning for redirect domains** — Control plane provisions certs on-demand when traffic arrives for a new domain. Customer's DNS must resolve to our NB IPs first.
- **DNSSEC** — Not implemented. Customer should evaluate whether DNSSEC is required for their redirect domains.
- **Monitoring/alerting** — No Prometheus/Grafana/PagerDuty integration. Recommend adding health check monitoring on the `/healthz` endpoints and NB status.
- **Backup/restore** — PG DBaaS handles automated backups. No application-level backup beyond the Object Storage `rules.json` snapshot.
- **WAF/bot protection** — Documented as an option (see `docs/akamai-integration.md`) but not deployed. Can be added by fronting the NB IPs with Akamai App & API Protector.

## Reference Documentation

- [README.md](README.md) — Architecture overview and quick start
- [DECISIONS.md](DECISIONS.md) — Why we made key technical choices
- [docs/ds2-beacon.md](docs/ds2-beacon.md) — DS2 beacon telemetry pattern (detailed)
- [docs/akamai-integration.md](docs/akamai-integration.md) — WAF/API Gateway integration options
- [docs/runbook.md](docs/runbook.md) — Operational procedures
- [web/static/openapi.json](web/static/openapi.json) — Full OpenAPI 3.0 spec for the REST API
