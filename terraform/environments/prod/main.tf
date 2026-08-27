# -----------------------------------------------------------------------------
# Production environment — multi-region TLD redirect engine
# Regions: us-ord (Chicago), us-iad (Washington DC)
# -----------------------------------------------------------------------------

terraform {
  required_providers {
    linode = {
      source  = "linode/linode"
      version = "~> 2.40"
    }
  }
  required_version = ">= 1.0"
}

provider "linode" {
  # Uses LINODE_TOKEN env var (PNC delegate token for the PNC account)
}

# DNS is owned by PNC — no Akamai/EdgeGrid provider needed. We output the two
# reserved IPs and PNC creates the redirect + test DNS records themselves.

# -----------------------------------------------------------------------------
# Region: us-ord (Chicago)
# -----------------------------------------------------------------------------

module "ord" {
  source = "../../modules/region"

  region                = "us-ord"
  region_label          = "ord"
  env_suffix            = "-PROD"
  ssh_public_key_path   = var.ssh_public_key_path
  admin_cidrs           = var.admin_cidrs
  ui_cidrs              = var.ui_cidrs
  data_instance_type    = var.data_instance_type
  data_instance_count   = 2
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
  env_suffix            = "-PROD"
  ssh_public_key_path   = var.ssh_public_key_path
  admin_cidrs           = var.admin_cidrs
  ui_cidrs              = var.ui_cidrs
  data_instance_type    = var.data_instance_type
  data_instance_count   = 2
  control_instance_type = var.control_instance_type
  tags                  = var.tags
}

# -----------------------------------------------------------------------------
# DNS — owned by PNC. Terraform does not create DNS records. The two reserved
# IPs are exposed via outputs; PNC points redirect + test DNS at them.
# -----------------------------------------------------------------------------

# -----------------------------------------------------------------------------
# Object Storage — cross-region redirect rule sync
#
# NOT managed here yet. The PNC delegate token currently lacks the Object
# Storage role (account_object_storage_creator), so the bucket is bootstrapped
# in Brian's personal account and wired via TLD_SYNC_* env on the instances.
# Once PNC grants the role, add the bucket + key resources here (Option A) or
# consume a PNC-provided S3 key (Option B), then migrate the sync target.
# -----------------------------------------------------------------------------
