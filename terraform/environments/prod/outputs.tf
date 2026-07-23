# -----------------------------------------------------------------------------
# Production environment outputs
# -----------------------------------------------------------------------------

# --- DNS targets for PNC ---
# NOTE: these are the NB-managed IPs today. Portability (reserved IP) is a
# pending decision — see ADR-010 draft. Do not hand these to PNC as permanent
# DNS targets until the portable-IP path is chosen.

output "dns_targets_for_pnc" {
  description = "NodeBalancer IPs PNC points redirect DNS at (portability pending — ADR-010)"
  value = {
    ord = module.ord.nodebalancer_ip
    iad = module.iad.nodebalancer_ip
  }
}

# --- ORD region ---

output "ord_nodebalancer_ip" {
  description = "ORD NodeBalancer public IPv4"
  value       = module.ord.nodebalancer_ip
}

output "ord_nodebalancer_hostname" {
  description = "ORD NodeBalancer hostname"
  value       = module.ord.nodebalancer_hostname
}

output "ord_data_public_ips" {
  description = "ORD data plane public IPs"
  value       = module.ord.data_public_ips
}

output "ord_data_private_ips" {
  description = "ORD data plane private IPs"
  value       = module.ord.data_private_ips
}

output "ord_control_public_ip" {
  description = "ORD control plane public IP"
  value       = module.ord.control_public_ip
}

output "ord_pg_host" {
  description = "ORD PostgreSQL host"
  value       = module.ord.pg_host
}

# --- IAD region ---

output "iad_nodebalancer_ip" {
  description = "IAD NodeBalancer public IPv4"
  value       = module.iad.nodebalancer_ip
}

output "iad_nodebalancer_hostname" {
  description = "IAD NodeBalancer hostname"
  value       = module.iad.nodebalancer_hostname
}

output "iad_data_public_ips" {
  description = "IAD data plane public IPs"
  value       = module.iad.data_public_ips
}

output "iad_data_private_ips" {
  description = "IAD data plane private IPs"
  value       = module.iad.data_private_ips
}

output "iad_control_public_ip" {
  description = "IAD control plane public IP"
  value       = module.iad.control_public_ip
}

output "iad_pg_host" {
  description = "IAD PostgreSQL host"
  value       = module.iad.pg_host
}

# --- Object Storage ---
# Not managed here yet — bootstrapped in Brian's personal account and wired via
# TLD_SYNC_* env until the PNC delegate gets account_object_storage_creator.
# See the sync bootstrap note in main.tf.
