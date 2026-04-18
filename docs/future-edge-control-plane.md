# Future Direction: Edge-Hosted Control Plane

## Concept

Replace the control plane VMs (currently 2x IaaS instances on Linode) with a WebAssembly-based control plane hosted at Akamai's edge. The Linode infrastructure shrinks to data plane serving only.

**Current:**
```
Admin Users → Control Plane VM (Linode IaaS) → Manages Data Plane VMs (Linode IaaS)
                                                        ↓ serve 301s
                                                   End Users
```

**Proposed:**
```
Admin Users → Akamai Function / Fermyon Spin app (edge)
                      ↓ writes rules to Object Storage (shared)
                      ↑ polls rules from Object Storage
              Data Plane VMs (Linode IaaS, raw serving only)
                      ↓ serve 301s
                   End Users
```

## Why Explore This

- **Eliminate IaaS control plane**: no OS patching, no systemd, no cert provisioning on the admin UI side (Akamai handles TLS at the edge)
- **Global low-latency admin UI**: control plane served from the nearest Akamai edge PoP
- **Simpler security posture**: no public-facing control plane VMs, no SSH management, no admin firewall rules
- **Integration with existing Akamai contract**: leverages infrastructure the customer already has

## Candidate Platforms

### Akamai EdgeWorkers / Akamai Functions
- JavaScript/TypeScript runtime at the edge
- Native integration with Akamai DataStream 2 (already in use)
- KV store (EdgeKV) or direct Object Storage for rule persistence
- Limitations: execution time caps, memory limits, no long-running processes

### Fermyon Spin (WebAssembly)
- WASM-based serverless runtime
- Can run on SpinKube or Fermyon Cloud
- More flexible than EdgeWorkers for larger logic
- Requires decision: host on Akamai Cloud Compute (Linode Kubernetes) or Fermyon's managed service

## Design Sketch

### What the edge control plane needs to do
1. Serve the admin UI (static SPA)
2. Handle REST API for domain/rule CRUD
3. Authenticate admin users (API token or OAuth/SSO integration)
4. Persist rules to Object Storage (replaces the PG write path)
5. Optionally: provision TLS certs for redirect domains (DNS-01 via Akamai Edge DNS API)

### What stays on Linode
- Data plane instances (redirect serving only)
- Managed PostgreSQL (analytics + request log only — no longer the rule source of truth)
- NodeBalancers (TCP passthrough)

### What shifts to shared storage
- Object Storage becomes the authoritative rule store
- Data plane polls ObjSt every 5s (already implemented)
- PG on data plane becomes a read-through cache + analytics sink only

### What goes away
- Control plane VMs (both regions)
- Control plane firewalls
- Per-region cert provisioning VMs (certs provisioned by edge workflow, stored in Vault)
- `-mode control` branch of the Go binary

## Open Questions

1. **Cert provisioning**: can EdgeWorkers / Spin call Let's Encrypt ACME and write certs to Vault? Or does this stay on an IaaS helper?
2. **Rule storage atomicity**: ObjSt PUT is atomic per object, but we need stronger consistency guarantees for the "multiple admins editing simultaneously" case. Consider conditional puts or a lightweight lock via Akamai KV.
3. **Migration path**: can we run both control planes in parallel (IaaS + edge) during cutover, writing to the same ObjSt bucket?
4. **Build toolchain**: EdgeWorkers (TS/JS), Spin (Rust/Go/TinyGo/JS) — pick based on team preference and performance needs.
5. **Admin auth**: edge-hosted auth flow (signed JWTs, API keys at edge, integration with enterprise IdP).

## Next Steps

1. POC the admin API on EdgeWorkers with a minimal CRUD surface (create domain, list domains)
2. Validate EdgeKV or EdgeWorker → ObjSt write latency
3. Determine cert provisioning strategy — edge-native or helper VM
4. Port the data plane unchanged (the current data plane already works with any rule source in ObjSt)
5. Deprecate IaaS control plane once edge parity is achieved

## References

- Current implementation: `internal/api/` (Go REST handlers), `web/static/` (vanilla JS SPA)
- Rule sync pattern: `internal/sync/sync.go` — already the integration point
- Certificate flow: ADR-005 in [DECISIONS.md](../DECISIONS.md)
