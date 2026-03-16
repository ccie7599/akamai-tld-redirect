# Scope — TLD Redirect Engine

## Tier

**Tier 2** — Pre-sales POC with production-grade architecture. Designed for customer handoff.

## Problem

Enterprise customer consolidating 2000+ legacy domains acquired through bank mergers. Current redirect infrastructure runs on F5 appliances in a single data center — single point of failure, no observability, no API for bulk management.

## Goals

- Multi-region redirect serving (2 regions, 4 data nodes) with automatic failover
- Separated control plane (admin API, cert provisioning) and data plane (redirect serving)
- Cross-region rule synchronization via Object Storage
- Automatic TLS for all domains via Let's Encrypt
- Per-redirect observability via Akamai DataStream 2 beacon pattern
- REST API + UI for domain/rule CRUD with bulk import/export
- Analytics pipeline with hourly rollups, top paths, referer tracking, inactive domain detection
- IaC for all infrastructure (Terraform)

## Non-Goals

- Full Akamai WAF/CDN integration (documented as option, not implemented)
- DNS management for customer domains (requires NS delegation from customer)
- DNSSEC support
- Multi-tenancy
- Automated domain discovery or crawling
- SLA commitments (projected availability documented, not contractually guaranteed)

## Exit Criteria

- [ ] Both regions serving redirects via NodeBalancer
- [ ] Cross-region sync verified (rule created on ORD appears on IAD <10s)
- [ ] Production Let's Encrypt certs on control planes
- [ ] DS2 beacon data flowing to webhook endpoint
- [ ] Admin UI accessible from customer network
- [ ] Documentation sufficient for delivery team handoff
