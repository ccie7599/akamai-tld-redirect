# -----------------------------------------------------------------------------
# Region module outputs
# -----------------------------------------------------------------------------

output "nodebalancer_ip" {
  description = "Public IPv4 of the region NodeBalancer"
  value       = linode_nodebalancer.data.ipv4
}

output "nodebalancer_hostname" {
  description = "Hostname of the region NodeBalancer"
  value       = linode_nodebalancer.data.hostname
}

output "nodebalancer_id" {
  description = "ID of the region NodeBalancer"
  value       = linode_nodebalancer.data.id
}

output "data_public_ips" {
  description = "Public IPs of data plane instances"
  value       = linode_instance.data[*].ip_address
}

output "data_private_ips" {
  description = "Private IPs of data plane instances"
  value       = linode_instance.data[*].private_ip_address
}

output "data_instance_ids" {
  description = "Instance IDs of data plane nodes"
  value       = linode_instance.data[*].id
}

output "control_public_ip" {
  description = "Public IP of the control plane instance"
  value       = linode_instance.control.ip_address
}

output "control_private_ip" {
  description = "Private IP of the control plane instance"
  value       = linode_instance.control.private_ip_address
}

output "control_instance_id" {
  description = "Instance ID of the control plane node"
  value       = linode_instance.control.id
}

output "pg_host" {
  description = "PostgreSQL DBaaS connection host"
  value       = linode_database_postgresql_v2.redirect.host_primary
}

output "pg_port" {
  description = "PostgreSQL DBaaS connection port"
  value       = linode_database_postgresql_v2.redirect.port
}

output "pg_ssl_connection" {
  description = "PostgreSQL DBaaS SSL connection string"
  value       = linode_database_postgresql_v2.redirect.ca_cert
  sensitive   = true
}

output "pg_root_password" {
  description = "PostgreSQL DBaaS root password"
  value       = linode_database_postgresql_v2.redirect.root_password
  sensitive   = true
}
