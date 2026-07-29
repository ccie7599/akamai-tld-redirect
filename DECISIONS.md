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

---

## ADR-008: No Edge-Hosted Control Plane for PNC Delivery

**Decision**: Deliver the control plane as the existing IaaS Go binary (`-mode control`) in PNC's compute environment. Do **not** build an edge-hosted (Akamai Functions / EdgeWorkers / Fermyon Spin) control plane for this engagement. The `docs/future-edge-control-plane.md` design is parked, not deleted.

**Context**: The deal closed and moved from pre-sales POC to delivery. Pre-teardown (Apr 18) we documented a future direction to move the admin control plane to Akamai's edge to shed IaaS operational burden. On revisiting for delivery, two facts kill it:

1. **Functions is assumed unavailable** in PNC's Akamai entitlements. We should not architect the delivery around a platform we may not have.
2. **Redirect traffic goes direct to origin.** There is no Akamai property in the redirect request path, so there is no edge for an EdgeWorker to execute on. An EW can only run on requests that traverse Akamai's edge — it can't help a direct-to-origin data plane. The only thing an edge worker could have hosted is the *admin* UI/API, which is low-traffic and does not justify a new runtime, toolchain, and cert/auth story.

**Rationale**: The IaaS Go control plane is proven — it's exactly what we demoed. Using it for delivery eliminates platform risk entirely (worst case = the known-good system) and removes the edge migration's open questions (cert provisioning at the edge, rule-store atomicity for concurrent admin edits, edge-native auth). The data plane already polls Object Storage for rules, so nothing about the data plane changes. If PNC later gains Functions and wants edge admin hosting as an optimization, the parked design remains valid — the Object Storage rule contract is the stable integration point.

**Revisit if**: PNC's redirect traffic is later fronted by an Akamai property (CDN/WAF), or Functions/EdgeWorkers becomes entitled and admin-UI latency/ops burden becomes a real pain point.

---

## ADR-009: Product Naming vs. Customer Configuration

**Decision**: The **product** keeps a generic name — `tld-redirect` binary, `cmd/tld-redirect/`, `TLD_*` env prefix, `/opt/tld-redirect/` install path, `tld-redirect-*` systemd units. **Customer-specific values** carry the customer prefix — Object Storage bucket (`pnc-redirect-sync`), redirect/admin domains, database, and resource tags. Customer identity lives in *config*, never in the binary or paths.

**Context**: The demo run drifted: instances were hand-deployed to `/opt/pnc-redirect/`, a `pnc-redirect` binary copy was created to match a mislabeled service file, and the sync bucket was renamed `pnc-redirect-sync`. The entire Mar 20 scale-down failure cascade (exit 203, env-prefix mismatch, service-path mismatch) traces to this half-finished rebrand — the code was always `tld-redirect`; only the deployed artifacts drifted.

**Rationale**: The redirect engine is a reusable asset intended for future customer engagements (per the TSA asset-extraction discipline). Baking `pnc` into the binary, paths, and env prefix would fork the product per customer and re-introduce exactly the drift that broke the demo. Keeping the product generic and pushing customer identity into config (env files, Terraform variables, tags) means one artifact deploys anywhere and deploys are reproducible. The uncommitted `pnc-redirect-sync` bucket rename is therefore correct-as-config; the code default staying `tld-redirect-sync` is correct-as-product (env overrides it per deployment).

**Consequence**: Reconcile all drift to this rule before deploying into PNC. Remove stray `pnc-redirect` binary/DB artifacts, keep the runbook on `/opt/tld-redirect/` + `TLD_*`, and treat any customer name in code as a bug.

---

## ADR-010: Portable Data-Plane Ingress — Reserved IP + NodeBalancer (create-and-import)

**Decision**: Front each region's data plane with a NodeBalancer that carries a portable **Reserved IP**. Because the Terraform provider cannot assign a reserved IP to an NB, the NB is created out-of-band via the Linode API (which does support it) and then `terraform import`-ed into the module's `linode_nodebalancer.data`. The reserved IP is managed **outside** Terraform state so it survives `terraform destroy`. Tooling: `scripts/provision-nb-reserved-ip.sh`.

**Context**: PNC owns DNS and wants a stable target they point to once and never re-point — even across an NB teardown/rebuild. Two candidate designs were considered: (A) NB + reserved IP, and (B) instances + reserved IP + keepalived VIP (no NB). Brian chose A to keep the NB's managed health-checking and failover between the two data VMs.

**The provider constraint (verified, not assumed)**: The Linode API accepts an `ipv4` field on NB create to attach an unassigned reserved IP, and this was confirmed live (reserve IP → `POST /nodebalancers {ipv4}` → NB carries the reserved address). But `linode_nodebalancer.ipv4` is **computed-only** in the Terraform provider — checked against the actual provider schema on both **2.41.2** and the latest **3.14.1** (`optional: null, computed: true`). The provider's `linode_reserved_ip_assignment` resource targets Linode *instances* only, not NBs. So pure-HCL reserved-IP-on-NB is not currently expressible.

**Rationale / validated workflow**:
1. Reserve an IP in the region (`POST /networking/ips {reserved:true}`) — lives outside TF.
2. Create the NB with `ipv4=<reserved>` (`POST /nodebalancers`).
3. `terraform import module.<region>.linode_nodebalancer.data <nb_id>` — verified that `terraform plan` then reports **"No changes"** (the computed `ipv4` does not drift).
4. `terraform apply` creates the NB configs, nodes, and firewall against the imported NB.
5. On rebuild, re-run the script with the **same** reserved IP; on full `terraform destroy` the NB releases the IP back to the account unassigned, and it reattaches on the next build — PNC's DNS target is stable.
6. Release (permanent): `DELETE /networking/reserved-ips/{address}`.

**Open dependency**: The PNC delegate token can read networking (`GET /networking/ips` → 200) but its permission to *reserve* an IP (`POST`) is unconfirmed — the delegate's IAM roles include no explicit networking/IP role. This may be a second role gap after Object Storage (ADR-009 / delegate scoping). Verify at first provisioning; if it 403s, PNC must grant the networking role (mirrors the `account_object_storage_creator` ask).

**Revisit if**: the Terraform provider makes `linode_nodebalancer.ipv4` settable (then fold the assignment into HCL and drop the create-and-import script), or the design moves to keepalived VIP (option B).

---

## ADR-011: Redirect Matching Semantics — Ported from project-redirect-functions

**Decision**: Replace the engine's simple first-match prefix logic with the richer, proven matcher from `project-redirect-functions`: explicit **match types** (exact | prefix), **longest-prefix-wins** priority, **preserve_path**, **preserve_query**, per-domain **case sensitivity**, and **path normalization**.

**Context**: The prior engine did first-rule-in-array prefix matching and always redirected to the flat `target_url`. For 2000+ vanity domains this was insufficient: no path preservation (`old.com/accounts/login` couldn't map to `new.com/accounts/login`), and rule precedence depended on array order rather than specificity (a latent correctness bug). `project-redirect-functions` already solved this for another engagement; PNC asked to bring that logic here. Full parity was chosen (see conversation) over a subset.

**Semantics** (Go reimplementation of the Rust/JS original, not a copy):
- **Normalization**: ensure leading `/`, collapse `//`, strip trailing `/` (except root); host lowercased + port stripped.
- **Resolution order**: exact match first (O(1) map), then the **longest** matching prefix (rules pre-sorted by path length at load; `priority` breaks ties between equal-length prefixes), else the domain's `default_url`.
- **preserve_path** (prefix rules): append the unmatched request-path suffix to the target (`/blog` → `new.com/articles`, request `/blog/2024/x` → `new.com/articles/2024/x`).
- **preserve_query**: forward the original query string.
- **case_sensitive** (per domain): when false, paths are folded to lowercase for matching; the original-case suffix is still used for `preserve_path`.

**Data model**: `redirect_rules` gains `match_type TEXT DEFAULT 'prefix'`, `preserve_path`, `preserve_query`; `domains` gains `case_sensitive DEFAULT true`. Added via idempotent `applyColumnMigrations()` (PG `ADD COLUMN IF NOT EXISTS`; SQLite tolerates duplicate-column) so existing deployments migrate in place. Fields flow through the admin API, single-create (defaults `case_sensitive` true, RFC-correct), and — importantly — the `rules.json` bulk import/export used for cross-region sync, so all nodes converge on identical matching.

**Divergences from the source / notes**:
- **match_type defaults to `prefix`** here (not the source's `exact`) to preserve the previous engine's behavior for rules that don't specify one; imports should set it explicitly. The seeded demo rules keep working unchanged.
- **Domain-level default redirect stays a flat redirect** to `default_url` (no path preservation) — preserving existing behavior. Path preservation is a per-rule opt-in.
- Covered by `internal/redirect/matcher_test.go` (exact, longest-prefix, preserve_path, preserve_query, case-insensitivity, normalization).

**Deploy note**: live nodes need a redeploy of the new binary; the column migrations run automatically at startup against the existing PG.
