# -----------------------------------------------------------------------------
# DNS module — A records on Akamai Edge DNS
# Creates A records for apex domains and control plane endpoints
# -----------------------------------------------------------------------------

terraform {
  required_providers {
    akamai = {
      source  = "akamai/akamai"
      version = ">= 6.0.0"
    }
  }
}

resource "akamai_dns_record" "a_record" {
  for_each = var.a_records

  zone       = var.zone
  name       = each.key
  recordtype = "A"
  ttl        = var.ttl
  target     = each.value
}
