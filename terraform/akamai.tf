# Akamai CDN Property — DS2 beacon for redirect analytics
# Receives fire-and-forget HTTP beacons from the redirect engine.
# Akamai edge returns 204 (no origin), DS2 captures request metadata.

locals {
  akamai_contract_id = "ctr_M-1YX7F61"
  akamai_group_id    = "grp_203183"
  akamai_product_id  = "prd_SPM"
  beacon_hostname    = "ds2-beacon.connected-cloud.io"
  akamai_email       = "bapley@akamai.com"
}

# --- CP Code ---

resource "akamai_cp_code" "beacon" {
  name        = local.beacon_hostname
  contract_id = local.akamai_contract_id
  group_id    = local.akamai_group_id
  product_id  = local.akamai_product_id
}

locals {
  beacon_cpcode_id = parseint(replace(akamai_cp_code.beacon.id, "cpc_", ""), 10)
}

# Edge hostname ds2-beacon.connected-cloud.io.edgekey.net (ehn_6156510)
# created out-of-band — Enhanced TLS, CPS enrollment 293468.

# --- Property ---

resource "akamai_property" "beacon" {
  name        = local.beacon_hostname
  product_id  = local.akamai_product_id
  contract_id = local.akamai_contract_id
  group_id    = local.akamai_group_id

  hostnames {
    cname_from             = local.beacon_hostname
    cname_to               = "${local.beacon_hostname}.edgekey.net"
    cert_provisioning_type = "CPS_MANAGED"
  }

  rule_format = "latest"
  rules       = templatefile("${path.root}/akamai/beacon-rules.json", {
    cpcode_id     = local.beacon_cpcode_id
    ds2_stream_id = var.ds2_stream_id
  })
}

# --- Activations ---

resource "akamai_property_activation" "beacon_staging" {
  property_id                    = akamai_property.beacon.id
  contact                        = [local.akamai_email]
  version                        = akamai_property.beacon.latest_version
  network                        = "STAGING"
  note                           = "DS2 beacon — redirect analytics"
  auto_acknowledge_rule_warnings = true
}

resource "akamai_property_activation" "beacon_production" {
  property_id                    = akamai_property.beacon.id
  contact                        = [local.akamai_email]
  version                        = akamai_property.beacon.latest_version
  network                        = "PRODUCTION"
  note                           = "DS2 beacon — production activation"
  auto_acknowledge_rule_warnings = true

  compliance_record {
    noncompliance_reason_no_production_traffic {}
  }
}

# --- DNS CNAME ---

resource "akamai_dns_record" "beacon_edge" {
  zone       = "connected-cloud.io"
  name       = local.beacon_hostname
  recordtype = "CNAME"
  ttl        = 300
  target     = ["${local.beacon_hostname}.edgekey.net."]
}

# --- DataStream 2 ---

resource "akamai_datastream" "redirect_beacon" {
  active      = true
  stream_name = "tld-redirect-ds2"
  contract_id = local.akamai_contract_id
  group_id    = local.akamai_group_id
  properties  = [akamai_property.beacon.id]

  dataset_fields = [
    1000, # CP code
    1002, # Request host
    1005, # Request method
    1006, # Request path (includes query string with redirect metrics)
    1008, # HTTP status code
    1013, # Request path (reqPath)
    1016, # User-Agent
    1017, # Status code (response)
    1102, # Turn around time (ms)
    2010, # Cache status (hit/miss)
    2012, # Client IP (Linode server IP)
    2013, # Country
    2014, # State
    3000  # Request ID
  ]

  delivery_configuration {
    format = "JSON"
    frequency {
      interval_in_secs = 30
    }
  }

  https_connector {
    display_name        = "tld-redirect-ds2-webhook"
    endpoint            = "https://${var.ds2_webhook_host}/api/ds2/webhook"
    authentication_type = "BASIC"
    user_name           = var.ds2_webhook_username
    password            = var.ds2_webhook_password
    content_type        = "application/json"
    compress_logs       = false
  }

  notification_emails = [local.akamai_email]

  depends_on = [akamai_property_activation.beacon_staging]
}

# Bootstrap: capture DS2 stream ID for property rules on next apply
resource "terraform_data" "ds2_bootstrap" {
  triggers_replace = [akamai_datastream.redirect_beacon.id]

  provisioner "local-exec" {
    command = "echo 'ds2_stream_id = ${akamai_datastream.redirect_beacon.id}' > ${path.root}/ds2.auto.tfvars"
  }
}

output "ds2_stream_id" {
  value = akamai_datastream.redirect_beacon.id
}

output "beacon_hostname" {
  value = local.beacon_hostname
}
