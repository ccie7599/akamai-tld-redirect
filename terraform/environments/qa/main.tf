# -----------------------------------------------------------------------------
# QA environment — downscaled mirror of prod (separate state, -QA labels)
# Dual region, 1 data VM/region, single-node PG, shared-CPU instances.
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
  # Uses LINODE_TOKEN env var (the full PNC token with compute + IP + Object Storage)
}

locals {
  # Shared-CPU instance for QA (2 GB); prod uses g6-dedicated-2.
  qa_instance_type = "g6-standard-1"
}

module "ord" {
  source = "../../modules/region"

  region                = "us-ord"
  region_label          = "ord"
  env_suffix            = "-QA"
  ssh_public_key_path   = var.ssh_public_key_path
  admin_cidrs           = var.admin_cidrs
  ui_cidrs              = var.ui_cidrs
  data_instance_type    = local.qa_instance_type
  data_instance_count   = 1 # downscaled: single data VM per region
  control_instance_type = local.qa_instance_type
  pg_cluster_size       = 1 # downscaled: single-node PG
  tags                  = var.tags
}

module "iad" {
  source = "../../modules/region"

  region                = "us-iad"
  region_label          = "iad"
  env_suffix            = "-QA"
  ssh_public_key_path   = var.ssh_public_key_path
  admin_cidrs           = var.admin_cidrs
  ui_cidrs              = var.ui_cidrs
  data_instance_type    = local.qa_instance_type
  data_instance_count   = 1
  control_instance_type = local.qa_instance_type
  pg_cluster_size       = 1
  tags                  = var.tags
}

# -----------------------------------------------------------------------------
# Object Storage — QA rule sync + shared cross-region cert store (ADR-003/012)
# -----------------------------------------------------------------------------

resource "linode_object_storage_bucket" "qa_sync" {
  label  = "pnc-redirect-qa-sync"
  region = "us-ord"
}

resource "linode_object_storage_key" "qa_sync" {
  label = "pnc-redirect-qa-sync-key"

  bucket_access {
    bucket_name = linode_object_storage_bucket.qa_sync.label
    region      = linode_object_storage_bucket.qa_sync.region
    permissions = "read_write"
  }
}
