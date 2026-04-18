# Operations Runbook

## Current State (2026-03-20)

Demo infrastructure has been torn down. This runbook documents the procedures for standing it back up from the repo.

## Architecture Summary

- **2 regions**: us-ord (Chicago), us-iad (Washington DC)
- **4 instances**: 1 control + 1 data per region (scaled from original 6)
- **2 Managed PG clusters**: 3-node each, one per region
- **2 NodeBalancers**: TCP passthrough on :80 and :443
- **1 Object Storage bucket**: cross-region rule sync
- **3 DNS A records**: apex redirect + 2 control plane hostnames

## Initial Deployment

### Prerequisites

- Linode API token in `LINODE_TOKEN` or `~/.config/linode-cli`
- Akamai EdgeGrid credentials in `~/.edgerc` (for DNS)
- SSH key at `~/.ssh/id_rsa.pub`
- Your public IPv4 for `admin_cidrs` in `terraform.tfvars`

### Stand Up Infrastructure

```bash
cd terraform/environments/prod
cp terraform.tfvars.example terraform.tfvars
# Edit terraform.tfvars and add your admin CIDR

terraform init
terraform plan -out=tfplan
terraform apply tfplan
```

PG clusters take ~10-15 min to provision. Everything else completes in <2 min.

### Build Binary

```bash
# Production build (CGO_ENABLED=0, PG-only)
make build-pg
```

### Deploy to Each Instance

For each instance (1 control + 1 data per region), you need:

1. Binary at `/opt/tld-redirect/tld-redirect`
2. Service file at `/etc/systemd/system/tld-redirect-{control,data}.service`
3. Environment file at `/opt/tld-redirect/env` with mode-specific variables

The automated script:

```bash
# For each instance, after building bin/tld-redirect
./scripts/deploy-multi.sh control <control-ip> <env-file>
./scripts/deploy-multi.sh data <data-ip> <env-file>
```

### Environment File Format

**Control plane** (`env` file on control instances):
```
TLD_DB_URL=postgresql://akmadmin:<pass>@<pg-ip>:25468/defaultdb?sslmode=require
TLD_ADMIN_TOKEN=<generate with: openssl rand -hex 16>
TLD_ADMIN_DOMAIN=tld-control-<region>.connected-cloud.io
TLD_SYNC_ENDPOINT=https://us-ord-1.linodeobjects.com
TLD_SYNC_BUCKET=pnc-redirect-sync
TLD_SYNC_ACCESS_KEY=<from terraform output>
TLD_SYNC_SECRET_KEY=<from terraform output>
TLD_SYNC_REGION=us-ord-1
```

**Data plane** (`env` file on data instances):
```
TLD_DB_URL=postgresql://akmadmin:<pass>@<pg-ip>:25468/defaultdb?sslmode=require
TLD_ADMIN_DOMAIN=tld-control-<region>.connected-cloud.io
TLD_DS2_ENDPOINT=https://ds2-beacon.connected-cloud.io/beacon
TLD_SYNC_ENDPOINT=https://us-ord-1.linodeobjects.com
TLD_SYNC_BUCKET=pnc-redirect-sync
TLD_SYNC_ACCESS_KEY=<from terraform output>
TLD_SYNC_SECRET_KEY=<from terraform output>
TLD_SYNC_REGION=us-ord-1
```

### Get PG Connection Details

Terraform state masks the password in outputs. Read the raw state:

```bash
cd terraform/environments/prod
python3 -c "
import json
with open('terraform.tfstate') as f:
    state = json.load(f)
for r in state['resources']:
    if r['type'] == 'linode_database_postgresql_v2':
        mod = r['module']
        a = r['instances'][0]['attributes']
        label = 'ORD' if 'ord' in mod else 'IAD'
        print(f'{label}_PG_HOST={a[\"host_primary\"]}  USER={a[\"root_username\"]}  PASS={a[\"root_password\"]}')
"
```

### Important: Use PG IP, Not Hostname

The PG hostname returns both A and AAAA records. Ubuntu 24.04's `getent hosts` prefers IPv6, but the PG allow_list only has IPv4 CIDRs. Resolve the IPv4 and use the IP directly:

```bash
dig +short <pg-hostname> | head -1
```

### Firewall Notes

- **Control plane FW**: needs port 80 open to 0.0.0.0/0 for ACME HTTP-01 challenges, and 443 for UI access
- **Data plane FW**: only allows NB private subnet (192.168.128.0/17) on 80/443 — do not open to public
- **NB FW**: 80/443 open to 0.0.0.0/0

Terraform-managed firewalls get reset on every apply. If you add rules manually via `linode-cli firewalls rules-update`, also update `terraform/modules/region/main.tf` to persist them.

### Seed Data

On the control plane after startup:

```bash
curl -sk -X POST "https://tld-control-<region>.connected-cloud.io/api/v1/import?token=<TOKEN>" \
  -H 'Content-Type: application/json' \
  -d @sample-data/redirects.json
```

## Common Operations

### Add a domain via API

```bash
curl -sk -X POST "https://tld-control-ord.connected-cloud.io/api/v1/domains?token=<TOKEN>" \
  -H 'Content-Type: application/json' \
  -d '{"name":"example.com","default_url":"https://www.target.com","status_code":301}'
```

The control plane writes to local PG, publishes to Object Storage, and all instances pick up the change within 10 seconds.

### Check service status across all instances

```bash
for IP in <all-instance-ips>; do
  echo -n "$IP: "; ssh root@$IP "systemctl is-active tld-redirect"
done
```

### View logs

```bash
ssh root@<IP> "journalctl -u tld-redirect -f"
```

### Force sync publish

Any mutation via the admin API triggers a sync publish. To force a re-sync without a change, update any domain (e.g., toggle enabled).

### Check PG connectivity

```bash
ssh root@<instance> "PGPASSWORD=<pass> psql 'postgresql://akmadmin@<pg-ip>:25468/defaultdb?sslmode=require' -c 'SELECT count(*) FROM domains;'"
```

## Failure Scenarios

### Data plane instance goes down

The NodeBalancer health check (TCP, 10s interval, 3 attempts) detects the failure and routes traffic to the remaining data instance in that region (if multi-data config) or the other region via DNS round-robin.

### Control plane goes down

Data plane continues serving redirects from its in-memory cache and local PG. New domain mutations are blocked until control is restored. The other region's control plane can be used for admin operations.

### PG goes down in one region

The 3-node managed PG cluster handles single-node failures automatically. If the entire cluster is unavailable, data plane serves from its in-memory cache (last loaded state). Control plane mutations fail.

### Cross-region sync failure

Object Storage polling logs errors every 5 seconds. Data plane continues serving from its last synced state. When connectivity restores, the next successful poll catches up immediately (ETag-based — no incremental state).

## Teardown

```bash
cd terraform/environments/prod
terraform destroy
```

Then verify no orphans:

```bash
linode-cli nodebalancers list --json | jq '.[] | select(.label | contains("tld"))'
linode-cli firewalls list --json | jq '.[] | select(.label | contains("tld"))'
linode-cli object-storage buckets-list | grep pnc-redirect
```

Delete any stragglers manually.

## Known Issues from the Demo Run

1. **Service file path mismatch**: manually-deployed instances used `/opt/pnc-redirect/` path (rebrand artifact). `scripts/deploy-multi.sh` now uses `/opt/tld-redirect/` consistently — greenfield deploys are fine.
2. **Env var prefix**: env files need `TLD_*` prefixes to match the service file's `${TLD_*}` references.
3. **PG v1 deprecated**: `linode_database_postgresql_v2` must be used, not v1.
4. **IPv6 PG connectivity**: see note above — use direct IPv4 in connection strings.
5. **Firewall persistence**: Terraform resets firewalls on apply, wiping manual `linode-cli` edits.
