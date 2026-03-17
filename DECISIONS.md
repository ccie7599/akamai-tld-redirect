# Architecture Decision Records

## ADR-001: Single Binary with Mode Flag

**Decision**: Use a single Go binary with `-mode control|data` flag rather than separate binaries.

**Context**: The control plane (admin API, cert provisioning, sync publishing) and data plane (redirect serving, beacon, analytics) have different responsibilities and security postures.

**Rationale**: A single binary simplifies builds, deployments, and version management. The mode flag branches startup logic cleanly — no shared code runs unnecessarily. Data plane never starts admin API; control plane never starts the redirect engine. This is simpler than maintaining two separate `cmd/` entry points that would share 90% of their dependency graph.

---

## ADR-002: PostgreSQL via pgx with SQLite Fallback

**Decision**: Use `jackc/pgx/v5` (pure Go) for production PostgreSQL, retain `mattn/go-sqlite3` for local dev.

**Context**: The original POC used SQLite. Multi-region deployment requires a shared database per region.

**Rationale**: pgx is pure Go (`CGO_ENABLED=0`) which simplifies cross-compilation and container builds. SQLite remains for local development (no PG dependency needed). The store layer uses a dialect abstraction — `q()` rewrites placeholders, custom scanners handle bool/timestamp differences, and `boolVal()` adapts boolean representation. The `-db-url` flag selects PG; the `-db` flag selects SQLite.

---

## ADR-003: Object Storage for Cross-Region Sync

**Decision**: Sync redirect rules across regions via S3-compatible Object Storage (Linode ObjSt) rather than PG logical replication or direct API calls.

**Context**: Linode Managed PostgreSQL does not support cross-region read replicas. We need rules from the ORD control plane to appear on IAD data plane instances.

**Rationale**: Object Storage is region-agnostic, cheap ($5/mo), and has a simple API. The sync pattern: control plane publishes `rules.json` on every mutation; all instances poll via `HEAD` (ETag check) every 5 seconds. Changed ETag triggers a `GET` + `BulkImportReplace` (UPSERT + orphan cleanup). This is eventually consistent (~5-10s) which is acceptable for redirect rule changes. No complex replication topology, no cross-region PG connections, no message broker.

---

## ADR-004: DS2 Beacon with Path-Encoded Metadata

**Decision**: Encode redirect metadata in the URL path of fire-and-forget HTTP beacons to an Akamai edge property backed by DataStream 2.

**Context**: We need per-redirect observability without adding latency or infrastructure (no log shippers, no separate metrics pipeline).

**Rationale**: DS2 strips query strings from `reqPath`, so we encode everything in the URL path. The edge property returns a synthetic 204 (no origin server). DS2 captures the request path and delivers batched JSON every 30 seconds. The beacon sender uses a buffered channel with non-blocking send — if the channel is full, beacons are silently dropped. Redirect latency is never impacted. See [docs/ds2-beacon.md](docs/ds2-beacon.md) for the full pattern.

---

## ADR-005: Certificate Storage — PG (current) vs HashiCorp Vault (production recommendation)

**Decision**: Current implementation stores certs in PostgreSQL via a `certmagic.Storage` adapter. For production deployments handling enterprise certificate inventory, HashiCorp Vault is the recommended storage backend.

**Context**: With 2000+ domains, certs must be provisioned on-demand and shared across instances. CertMagic's default file storage doesn't work across multiple nodes. Certificate private keys are sensitive material that enterprises typically require to be managed by a dedicated secrets management system.

**Current implementation (POC / lower-grade production)**: The `cert_store` table in PG implements `certmagic.Storage` (Lock via PG advisory locks, CRUD via simple queries). Control plane runs the full CertMagic provisioner; data plane instantiates CertMagic in loader-only mode (reads certs from PG, serves TLS, but never initiates ACME challenges). This prevents race conditions where multiple instances simultaneously attempt HTTP-01 challenges for the same domain.

This approach works and is operationally simple, but stores private key material alongside application data in the same database — acceptable for POC and lower-sensitivity production workloads, but not aligned with enterprise secret management practices.

**Production recommendation — HashiCorp Vault**:

For production deployments, implement a `certmagic.Storage` adapter backed by Vault's KV v2 secrets engine:

- **Key path**: `secret/certs/{domain}/privkey`, `secret/certs/{domain}/cert`, `secret/certs/{domain}/chain`
- **Locking**: Vault's system lock or Consul-backed distributed locks
- **Access control**: Vault policies scoped per service — control plane gets read/write, data plane gets read-only
- **Audit**: Vault's audit log provides a complete record of every cert read/write — critical for compliance
- **Rotation**: Vault's lease/TTL mechanism can enforce cert rotation windows
- **Encryption at rest**: Vault encrypts all stored secrets with its barrier key (AES-256-GCM)

Implementation path: create `internal/certs/vault_storage.go` implementing `certmagic.Storage` against the Vault HTTP API or `hashicorp/vault/api` Go client. The `-cert-backend` flag would select between `pg` (current) and `vault` (with `-vault-addr`, `-vault-token` or `-vault-role` for AppRole auth).

Most enterprises evaluating this solution already have Vault deployed. If they don't, the PG storage backend is a reasonable starting point that can be migrated to Vault later — the `certmagic.Storage` interface abstracts the backend completely.

---

## ADR-006: NodeBalancer TCP Passthrough

**Decision**: NodeBalancer operates in TCP passthrough mode (not HTTP/HTTPS termination).

**Context**: NodeBalancers support a maximum of 10 SSL certificates. We need TLS for 2000+ domains.

**Rationale**: TCP passthrough forwards raw TCP to the data plane instances, which handle TLS termination via CertMagic. This removes the certificate limit and keeps TLS management in the application layer where CertMagic can provision on-demand. Health checks use TCP connection probes (not HTTP) since the NB doesn't terminate TLS.

---

## ADR-007: Firewall Separation

**Decision**: Separate Cloud Firewalls for control plane and data plane with distinct inbound rules.

**Context**: Control plane handles admin operations (sensitive). Data plane handles public redirect traffic.

**Rationale**: Data plane firewall only allows inbound from the NodeBalancer private subnet (`192.168.128.0/17`) on ports 80/443 and admin CIDRs on port 22. Control plane firewall allows admin CIDRs on 443/22 and `0.0.0.0/0` on port 80 (for ACME HTTP-01 challenges). This ensures a compromised data plane instance can't access admin APIs, and public traffic can't reach data plane instances directly (must go through NB).
