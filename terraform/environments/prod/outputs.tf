# -----------------------------------------------------------------------------
# Production environment outputs
# -----------------------------------------------------------------------------

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

output "objst_bucket" {
  description = "Object Storage bucket label for redirect rule sync"
  value       = linode_object_storage_bucket.redirect_sync.label
}

output "objst_endpoint" {
  description = "Object Storage bucket hostname"
  value       = linode_object_storage_bucket.redirect_sync.hostname
}

output "objst_access_key" {
  description = "Object Storage access key"
  value       = linode_object_storage_key.redirect_sync.access_key
}

output "objst_secret_key" {
  description = "Object Storage secret key"
  value       = linode_object_storage_key.redirect_sync.secret_key
  sensitive   = true
}

# Akamai DS2 outputs managed in terraform/ root state
