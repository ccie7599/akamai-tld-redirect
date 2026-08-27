# --- QA DNS targets (managed NB IPs — QA does not use reserved IPs) ---

output "qa_dns_targets" {
  description = "QA NodeBalancer IPs for PNC to point QA test DNS at"
  value = {
    ord = module.ord.nodebalancer_ip
    iad = module.iad.nodebalancer_ip
  }
}

output "ord_control_public_ip" { value = module.ord.control_public_ip }
output "iad_control_public_ip" { value = module.iad.control_public_ip }
output "ord_data_public_ips" { value = module.ord.data_public_ips }
output "iad_data_public_ips" { value = module.iad.data_public_ips }

# --- Object Storage (QA sync + cert store) ---
output "qa_objst_bucket"   { value = linode_object_storage_bucket.qa_sync.label }
output "qa_objst_endpoint" { value = linode_object_storage_bucket.qa_sync.hostname }
output "qa_objst_access_key" { value = linode_object_storage_key.qa_sync.access_key }
output "qa_objst_secret_key" {
  value     = linode_object_storage_key.qa_sync.secret_key
  sensitive = true
}
