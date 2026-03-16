# Operations Runbook

## Instance Map

| Role | Region | IP | Hostname |
|------|--------|----|----------|
| Control | us-ord | 172.237.132.8 | tld-control-ord.connected-cloud.io |
| Control | us-iad | 172.234.130.4 | tld-control-iad.connected-cloud.io |
| Data | us-ord | 172.237.132.38 | behind NB 172.238.165.172 |
| Data | us-ord | 172.234.17.252 | behind NB 172.238.165.172 |
| Data | us-iad | 172.237.164.203 | behind NB 139.144.195.77 |
| Data | us-iad | 172.234.130.6 | behind NB 139.144.195.77 |

## Common Operations

### Deploy a new binary

```bash
# Build (CGO_ENABLED=0 for PG-only production binary)
make build-pg

# Deploy to a specific instance
scp bin/tld-redirect root@<IP>:/opt/tld-redirect/tld-redirect
ssh root@<IP> "setcap cap_net_bind_service=+ep /opt/tld-redirect/tld-redirect && systemctl restart tld-redirect"
```

### Check service status across all instances

```bash
for IP in 172.237.132.8 172.237.132.38 172.234.17.252 172.234.130.4 172.237.164.203 172.234.130.6; do
  echo -n "$IP: "; ssh root@$IP "systemctl is-active tld-redirect"
done
```

### View logs

```bash
ssh root@<IP> "journalctl -u tld-redirect -f"
```

### Add a domain via API

```bash
curl -sk -X POST "https://tld-control-ord.connected-cloud.io/api/v1/domains?token=<TOKEN>" \
  -H 'Content-Type: application/json' \
  -d '{"name":"example.com","default_url":"https://www.target.com","status_code":301}'
```

The control plane writes to local PG, publishes to Object Storage, and all instances pick up the change within 10 seconds.

### Bulk import

```bash
curl -sk -X POST "https://tld-control-ord.connected-cloud.io/api/v1/import?token=<TOKEN>" \
  -H 'Content-Type: application/json' \
  -d @domains.json
```

### Force sync publish

Any mutation via the admin API triggers a sync publish. To force a re-sync without a change, update any domain (e.g., toggle enabled).

### Check PG connectivity

```bash
ssh root@172.237.132.8 "PGPASSWORD=<pass> psql 'postgresql://akmadmin@<PG_IP>:25468/defaultdb?sslmode=require' -c 'SELECT count(*) FROM domains;'"
```

## Failure Scenarios

### Data plane instance goes down

The NodeBalancer health check (TCP, 10s interval, 3 attempts) detects the failure and routes traffic to the remaining data instance in that region. No manual intervention needed.

### Control plane goes down

Data plane continues serving redirects from its in-memory cache and local PG. New domain mutations are blocked until control is restored. The other region's control plane can be used for admin operations.

### PG goes down in one region

The 3-node managed PG cluster handles single-node failures automatically. If the entire cluster is unavailable, data plane serves from its in-memory cache (last loaded state). Control plane mutations fail with a database error.

### Cross-region sync failure

Object Storage polling logs errors every 5 seconds. Data plane continues serving from its last synced state. When connectivity restores, the next successful poll catches up immediately (ETag-based — no incremental state).

## Terraform

```bash
cd terraform/environments/prod
terraform plan    # Review changes
terraform apply   # Apply (PG changes take ~15 min)
```

**Warning**: Terraform manages firewall rules. Manual firewall changes via `linode-cli` will be overwritten on next apply. Add permanent rules to `admin_cidrs` in `terraform.tfvars`.
