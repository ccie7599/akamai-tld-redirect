# -----------------------------------------------------------------------------
# Production environment — multi-region TLD redirect engine
# Regions: us-ord (Chicago), us-iad (Washington DC)
# -----------------------------------------------------------------------------

terraform {
  required_providers {
    linode = {
      source  = "linode/linode"
      version = "~> 2.36"
    }
    akamai = {
      source  = "akamai/akamai"
      version = ">= 6.0.0"
    }
  }
  required_version = ">= 1.0"
}

provider "linode" {
  # Uses LINODE_TOKEN env var
}

provider "akamai" {
  # Uses ~/.edgerc [default] section
}

# -----------------------------------------------------------------------------
# Region: us-ord (Chicago)
# -----------------------------------------------------------------------------

module "ord" {
  source = "../../modules/region"

  region                = "us-ord"
  region_label          = "ord"
  ssh_public_key_path   = var.ssh_public_key_path
  admin_cidrs           = var.admin_cidrs
  data_instance_type    = var.data_instance_type
  control_instance_type = var.control_instance_type
  tags                  = var.tags
}

# -----------------------------------------------------------------------------
# Region: us-iad (Washington DC)
# -----------------------------------------------------------------------------

module "iad" {
  source = "../../modules/region"

  region                = "us-iad"
  region_label          = "iad"
  ssh_public_key_path   = var.ssh_public_key_path
  admin_cidrs           = var.admin_cidrs
  data_instance_type    = var.data_instance_type
  control_instance_type = var.control_instance_type
  tags                  = var.tags
}

# -----------------------------------------------------------------------------
# DNS — A records for redirect domains and control plane
# -----------------------------------------------------------------------------

module "dns" {
  source = "../../modules/dns"

  a_records = {
    # Redirect data plane — both region NB IPs for GTM/failover
    "tld-redirect.connected-cloud.io" = [
      module.ord.nodebalancer_ip,
      module.iad.nodebalancer_ip,
    ]

    # Per-region control plane direct access
    "tld-control-ord.connected-cloud.io" = [module.ord.control_public_ip]
    "tld-control-iad.connected-cloud.io" = [module.iad.control_public_ip]
  }
}

# -----------------------------------------------------------------------------
# Object Storage — cross-region redirect rule sync bucket
# -----------------------------------------------------------------------------

resource "linode_object_storage_bucket" "redirect_sync" {
  label  = "tld-redirect-sync"
  region = "us-ord"
}

resource "linode_object_storage_key" "redirect_sync" {
  label = "tld-redirect-sync-key"

  bucket_access {
    bucket_name = linode_object_storage_bucket.redirect_sync.label
    region      = linode_object_storage_bucket.redirect_sync.region
    permissions = "read_write"
  }
}
