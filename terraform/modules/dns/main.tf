# -----------------------------------------------------------------------------
# DNS module — A records for apex domains and control plane endpoints
#
# This reference implementation uses Akamai Edge DNS. The redirect engine
# is DNS-provider agnostic — replace this module with any provider that
# supports A records (Cloudflare, Route 53, Linode DNS Manager, etc.).
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
