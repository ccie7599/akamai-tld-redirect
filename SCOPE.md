# Scope — TLD Redirect Engine (PNC Delivery)

## Tier

**Tier 2 — Delivery.** The pre-sales POC is complete and the deal has closed. This
engagement is now a production build inside PNC's environment. Brian owns the build.

## Problem

PNC is consolidating 2000+ legacy domains acquired through bank mergers. The current
redirect infrastructure runs on F5 appliances in a single data center — single point of
failure, no observability, no API for bulk management. The delivery replaces F5 with the
TLD Redirect Engine running in PNC's compute environment.

## Delivery Architecture (decided)

- **Control plane and data plane both run on IaaS** in PNC's compute environment — the
  proven Go binary (`-mode control|data`), the same build demoed pre-sales.
- **No edge control plane.** Akamai Functions/EdgeWorkers is out of scope for delivery:
  Functions is assumed unavailable in PNC's entitlements, and redirect traffic goes direct
  to origin (no Akamai edge in the request path for an EdgeWorker to execute on). See
  ADR-008. The `docs/future-edge-control-plane.md` design is parked, not deleted.
- **Product vs. customer config**: the product stays generically named (`tld-redirect`
  binary, `TLD_*` env prefix, `/opt/tld-redirect/` paths). Customer-specific values
  (Object Storage bucket, redirect/admin domains, DB, resource tags) carry `pnc-`. See
  ADR-009.

## Goals

- Deploy control + data plane into PNC's compute environment (region/topology TBD on
  access) with automatic failover
- Cross-region rule synchronization via Object Storage (already implemented)
- Automatic TLS for all domains via Let's Encrypt (CertMagic; cert backend per ADR-005)
- REST API + UI for domain/rule CRUD with bulk import/export
- Per-redirect observability via DS2 beacon pattern (or PNC-preferred equivalent if
  DataStream 2 is not in their contract — confirm on access)
- Analytics pipeline with hourly rollups, top paths, referer tracking, inactive domain
  detection
- IaC for all infrastructure (Terraform), parameterized for PNC's environment
- Cutover plan from F5 (parallel-run, DNS migration, rollback)

## Non-Goals

- Edge-hosted control plane (Akamai Functions / EdgeWorkers / Fermyon Spin) — parked
- Full Akamai WAF/CDN integration (documented as option, not implemented)
- DNS management for customer domains (PNC owns NS delegation)
- DNSSEC support
- Multi-tenancy
- Automated domain discovery or crawling

## Open — resolve on environment access

- [ ] PNC compute environment details: provider, regions, network constraints, VM sizing
- [ ] Akamai entitlements: is DataStream 2 available for the beacon pattern? If not, pick
      an alternative observability sink
- [ ] Object Storage: PNC-provided S3-compatible bucket vs. Linode ObjSt
- [ ] Managed PostgreSQL availability and topology in PNC's environment
- [ ] Cert storage backend: PG (current) vs. HashiCorp Vault (ADR-005) — does PNC run Vault?
- [ ] Admin authentication: query-string token (current) vs. PNC enterprise IdP/SSO
- [ ] Firewall / IP-restriction model in PNC's network

## Exit Criteria (delivery)

- [ ] Control + data plane deployed and serving redirects in PNC's environment
- [ ] Cross-region (or cross-AZ) sync verified
- [ ] Production TLS certs provisioned for the initial domain batch
- [ ] Observability data flowing to the agreed sink
- [ ] Admin UI/API accessible per PNC's network policy with agreed auth
- [ ] Bulk import of PNC's domain inventory validated at scale
- [ ] F5 cutover plan documented and dry-run tested
- [ ] Handoff docs updated for PNC's operations team (HANDOFF.md, runbook)

## Pre-access preparation (this phase, no PNC env yet)

- [x] Delivery scope locked (this doc)
- [x] Edge control plane decision recorded (ADR-008)
- [ ] Go control plane hardened as the delivery baseline — see hardening backlog
- [ ] Terraform parameterized so environment specifics are variables, not hardcoded
- [ ] Naming/config drift eliminated so deploys are reproducible (root cause of the Mar 20
      demo failures)
