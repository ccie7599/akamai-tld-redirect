output "instance_ip" {
  description = "Public IP of the redirect server"
  value       = linode_instance.redirect.ip_address
}

output "instance_id" {
  description = "Linode instance ID"
  value       = linode_instance.redirect.id
}

output "instance_label" {
  description = "Linode instance label"
  value       = linode_instance.redirect.label
}
