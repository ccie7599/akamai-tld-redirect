# Operations Runbook

## Current State (2026-07-23)

**Live in PNC's Linode account** (provisioned via the PNC delegate token). Managed-IP
NodeBalancers; portable Reserved IPs are deferred until PNC grants the delegate a
networking/IP role (see Pending Migrations). Object Storage rule-sync is bootstrapped in
Brian's personal account until PNC grants the Object Storage role.

| | ORD | IAD |
|---|---|---|
| NodeBalancer IP (interim DNS target) | 172.238.165.194 | 172.234.156.196 |
| Control plane | 172.233.217.89 | 172.237.161.169 |
| Data VMs | 172.233.217.57, .59 | 139.144.204.108, 172.237.161.125 |
| PG host | a500088-…akamaidb.net | a500089-…akamaidb.net |
| Admin domain (reverse DNS) | 172-233-217-89.ip.linodeusercontent.com | 172-237-161-169.ip.linodeusercontent.com |

## Architecture Summary

- **2 regions**: us-ord (Chicago), us-iad (Washington DC)
- **6 instances**: 1 control + 2 data per region (data pair fails over behind the NB)
- **2 Managed PG clusters**: 3-node each, one per region
- **2 NodeBalancers**: TCP passthrough on :80 and :443 (managed IP today; Reserved IP pending)
- **Object Storage**: cross-region rule sync — currently the personal-account bootstrap
  bucket `tld-redirect-sync-bootstrap` (us-ord-1)
- **DNS**: owned by PNC. Terraform creates no DNS records. Control-plane admin access uses
  Linode reverse-DNS hostnames (`<ip>.ip.linodeusercontent.com`), which resolve and let
  CertMagic issue real Let's Encrypt certs without any PNC DNS.

## Initial Deployment

### Prerequisites

- **PNC delegate token** exported as `LINODE_TOKEN` (compute only — the delegate lacks
  Object Storage and networking/IP roles; see Pending Migrations)
- SSH key at `~/.ssh/id_rsa.pub`
- The deploy box's public IPv4 in `admin_cidrs` (needed for SSH to instances + PG allow_list)
- **No** Akamai EdgeGrid credential needed — PNC owns DNS, the akamai provider was dropped

### Stand Up Infrastructure

```bash
cd terraform/environments/prod
# terraform.tfvars: admin_cidrs = ["<deploy-box-ip>/32", ...]
export LINODE_TOKEN=<pnc-delegate-token>
terraform init
terraform plan -out=tfplan
terraform apply tfplan
```

PG clusters take ~10-15 min to provision. Everything else completes in <2 min. Expect 28
resources (14 per region: PG, 3 firewalls, 3 instances, NB + 2 configs + 4 nodes).

Data-plane firewall is unchanged from the NB design — the NB reaches backends over the
private subnet (192.168.128.0/17), so no public 80/443 on the VMs.

### Build Binary

```bash
make build-pg   # CGO_ENABLED=0, PG-only
```

### Object Storage (rule sync)

The PNC delegate cannot create Object Storage (403 — no role). Until PNC grants
`account_object_storage_creator`, the sync bucket + a bucket-scoped read_write key live in
Brian's **personal** account. To (re)create the bootstrap:

```bash
# bucket (via boto3 against us-ord-1) + scoped key (via linode-cli, personal profile)
linode-cli object-storage keys-create --label tld-redirect-sync-bootstrap \
  --bucket_access.bucket_name tld-redirect-sync-bootstrap \
  --bucket_access.region us-ord --bucket_access.permissions read_write
```

Feed the endpoint/bucket/keys into the `TLD_SYNC_*` env vars below.

### Deploy to Each Instance

```bash
export LINODE_TOKEN=<pnc-delegate-token>   # deploy-multi.sh rebuilds the binary
./scripts/deploy-multi.sh control <control-ip> <env-file>
./scripts/deploy-multi.sh data    <data-ip>    <env-file>   # once per data VM
```

Installs binary at `/opt/tld-redirect/tld-redirect`, service at
`/etc/systemd/system/tld-redirect-{control,data}.service`, env at `/opt/tld-redirect/env`.

### Environment File Format

**Control plane:**
```
TLD_DB_URL=postgresql://akmadmin:<pass>@<pg-ipv4>:20253/defaultdb?sslmode=require
TLD_ADMIN_TOKEN=<openssl rand -hex 32>          # unset/"changeme" is refused (fail-closed)
TLD_ADMIN_DOMAIN=<ip-dashed>.ip.linodeusercontent.com   # resolvable -> real ACME cert
TLD_SYNC_ENDPOINT=https://us-ord-1.linodeobjects.com
TLD_SYNC_BUCKET=tld-redirect-sync-bootstrap     # -> pnc-redirect-sync after migration
TLD_SYNC_ACCESS_KEY=<personal bootstrap access key>
TLD_SYNC_SECRET_KEY=<personal bootstrap secret key>
TLD_SYNC_REGION=us-ord-1
```

**Data plane:** same as above minus `TLD_ADMIN_TOKEN`. `TLD_DS2_ENDPOINT` is intentionally
unset — observability sink is TBD with PNC.

Note: **PG port is 20253** (read it from state — do not assume) and **use the PG IPv4, not
the hostname** (Ubuntu 24.04 prefers the AAAA record, which isn't in the allow_list).

### Get PG Connection Details

```bash
cd terraform/environments/prod
python3 -c "
import json
state = json.load(open('terraform.tfstate'))
for r in state['resources']:
    if r['type'] == 'linode_database_postgresql_v2':
        a = r['instances'][0]['attributes']
        label = 'ORD' if 'ord' in r['module'] else 'IAD'
        print(f'{label}: host={a[\"host_primary\"]} port={a[\"port\"]} user={a[\"root_username\"]} pass={a[\"root_password\"]}')
"
dig +short <pg-hostname> | head -1   # resolve the IPv4 for TLD_DB_URL
```

### Seed Data

Seed via **one** control plane (it publishes to Object Storage; all regions sync in ~10s —
don't seed both control planes or they double-write the same key):

```bash
curl -s -X POST "https://<ord-admin-domain>/api/v1/import?token=<TOKEN>" \
  -H 'Content-Type: application/json' -d @sample-data/redirects.json
```

### Verify

```bash
# Redirect serves for a seeded domain (HTTP :80 serves redirects directly for known hosts)
curl -s -o /dev/null -w "%{http_code} %{redirect_url}\n" \
  -H "Host: old-brand-financial.example.com" http://<nb-ip>/
# Test both region NBs — IAD serving proves the cross-region sync path.
```

## Pending Migrations (blocked on PNC delegate roles)

### 1. Reserved IPs — portable DNS targets (ADR-010)

The delegate can't reserve IPs (`POST /networking/ips` → 403). Once PNC grants the
networking role (likely `account_ip_creator` — confirm in their IAM catalog), swap each NB
to a Reserved IP **before** PNC points DNS. The provider can't set NB `ipv4`, so create the
NB with the reserved IP via the API and import it:

```bash
LINODE_TOKEN=<delegate> ./scripts/provision-nb-reserved-ip.sh us-ord tld-ord-nb module.ord
LINODE_TOKEN=<delegate> ./scripts/provision-nb-reserved-ip.sh us-iad tld-iad-nb module.iad
# then, per the script output:
terraform import 'module.<region>.linode_nodebalancer.data' <nb_id>
terraform apply
```

Hand PNC the two Reserved IPs as the permanent DNS targets. **Do not give them the current
NB IPs** — those change on NB rebuild.

### 2. Object Storage — PNC-owned bucket

Once PNC grants `account_object_storage_creator`, create `pnc-redirect-sync` in their
account (or take PNC-provided S3 keys), update `TLD_SYNC_*` on every instance, restart, and
verify sync. Then revoke the personal bootstrap key and delete the bootstrap bucket.

### 3. Redirect-domain TLS + observability

- Real per-domain certs need PNC to point test/prod DNS at the data plane (CertMagic issues
  on demand).
- Wire `TLD_DS2_ENDPOINT` (or a PNC-preferred sink) once observability is confirmed.

## Common Operations

### Add a domain via API
```bash
curl -s -X POST "https://<ord-admin-domain>/api/v1/domains?token=<TOKEN>" \
  -H 'Content-Type: application/json' \
  -d '{"name":"example.com","default_url":"https://www.target.com","status_code":301}'
```
Control writes to local PG, publishes to Object Storage, all instances converge in ~10s.

### Fleet status
```bash
for IP in 172.233.217.89 172.233.217.57 172.233.217.59 172.237.161.169 139.144.204.108 172.237.161.125; do
  echo -n "$IP: "; ssh root@$IP "systemctl is-active 'tld-redirect-*'"
done
```

### Logs / force re-sync
```bash
ssh root@<IP> "journalctl -u tld-redirect-control -f"   # or -data
# force re-sync: toggle any domain's enabled state via the admin API
```

## Failure Scenarios

- **Data VM down**: NB TCP health check (10s, 3 attempts) routes to the other data VM in region.
- **Control down**: data planes keep serving from cache/PG; mutations blocked; use the other region's control.
- **PG cluster down**: single-node failures auto-handled (3-node); full outage → data serves last-synced state, mutations fail.
- **Sync failure**: ObjSt polling logs errors every 5s; data serves last state; recovers on next successful poll (ETag-based).

## Teardown

```bash
cd terraform/environments/prod
export LINODE_TOKEN=<pnc-delegate-token>
terraform destroy
```

Then verify no orphans in PNC's account (delegate-visible resource types):

```bash
linode-cli --profile <pnc> nodebalancers list --json | jq '.[] | select(.label | startswith("tld-"))'
linode-cli --profile <pnc> firewalls list --json | jq '.[] | select(.label | startswith("tld-"))'
```

If Reserved IPs were assigned, release them (permanent): `DELETE /networking/reserved-ips/{address}`.
The personal-account bootstrap bucket/key are separate — clean those up in the personal account.

## Known Issues

1. **PG port + IPv4**: port is 20253 (from state), and connection strings must use the PG
   IPv4 — Ubuntu 24.04 prefers the AAAA record, which isn't in the allow_list.
2. **Reserved-IP-on-NB not in Terraform**: provider `linode_nodebalancer.ipv4` is
   computed-only (2.41.2 and 3.14.1) — use the create-and-import flow (ADR-010).
3. **Delegate role gaps**: no Object Storage or networking/IP role — hence the personal
   bootstrap and managed-IP NBs. Both are Pending Migrations above.
4. **Fail-closed admin token**: control plane refuses to start with an unset or `changeme`
   token — set a strong `TLD_ADMIN_TOKEN`.
5. **Firewall persistence**: Terraform resets firewalls on apply; fold any manual rules into
   `terraform/modules/region/main.tf`.
