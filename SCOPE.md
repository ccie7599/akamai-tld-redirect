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

- [x] Both regions serving redirects via NodeBalancer (verified 2026-03-16: ORD 172.238.165.172, IAD 139.144.195.77)
- [x] Cross-region sync verified (rule created on ORD appears on IAD <10s, verified 2026-03-16)
- [x] Production Let's Encrypt certs on control planes (ORD: E7, IAD: E8, expires 2026-06-14)
- [x] DS2 beacon data flowing to webhook endpoint (beacon property active, 600+ beacons captured)
- [x] Admin UI accessible from customer network (ORD + IAD both HTTP 200, home IP in firewall)
- [x] Documentation sufficient for delivery team handoff (README, DECISIONS, HANDOFF, runbook, ds2-beacon, akamai-integration)
