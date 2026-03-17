# -----------------------------------------------------------------------------
# Region module — creates the full per-region stack for the TLD redirect engine
# Resources: PG DBaaS, NodeBalancer, data instances, control instance, firewalls
# -----------------------------------------------------------------------------

terraform {
  required_providers {
    linode = {
      source  = "linode/linode"
      version = "~> 2.36"
    }
  }
}

locals {
  prefix     = "tld-${var.region_label}"
  ssh_pubkey = trimspace(file(var.ssh_public_key_path))
  cloud_init = base64encode(file("${path.module}/cloud-init.yaml"))
}

# -----------------------------------------------------------------------------
# PostgreSQL DBaaS
# -----------------------------------------------------------------------------

resource "linode_database_postgresql_v2" "redirect" {
  label     = "${local.prefix}-pg"
  region    = var.region
  engine_id = var.pg_engine_id
  type      = var.pg_engine_type

  cluster_size = var.pg_cluster_size

  allow_list = concat(
    [for inst in linode_instance.data : "${inst.private_ip_address}/32"],
    [for inst in linode_instance.data : "${inst.ip_address}/32"],
    ["${linode_instance.control.private_ip_address}/32"],
    ["${linode_instance.control.ip_address}/32"],
    var.admin_cidrs,
  )
}

# -----------------------------------------------------------------------------
# Data plane instances
# -----------------------------------------------------------------------------

resource "linode_instance" "data" {
  count = var.data_instance_count

  label           = "${local.prefix}-data-${count.index}"
  region          = var.region
  type            = var.data_instance_type
  image           = "linode/ubuntu24.04"
  private_ip      = true
  authorized_keys = [local.ssh_pubkey]
  tags            = var.tags

  metadata {
    user_data = local.cloud_init
  }
}

# -----------------------------------------------------------------------------
# Control plane instance
# -----------------------------------------------------------------------------

resource "linode_instance" "control" {
  label           = "${local.prefix}-control"
  region          = var.region
  type            = var.control_instance_type
  image           = "linode/ubuntu24.04"
  private_ip      = true
  authorized_keys = [local.ssh_pubkey]
  tags            = var.tags

  metadata {
    user_data = local.cloud_init
  }
}

# -----------------------------------------------------------------------------
# NodeBalancer — TCP passthrough for data plane
# -----------------------------------------------------------------------------

resource "linode_nodebalancer" "data" {
  label  = "${local.prefix}-nb"
  region = var.region
  tags   = var.tags
}

resource "linode_nodebalancer_config" "https" {
  nodebalancer_id = linode_nodebalancer.data.id
  port            = 443
  protocol        = "tcp"
  algorithm       = "roundrobin"
  check           = "connection"
  check_interval  = 10
  check_timeout   = 5
  check_attempts  = 3
  stickiness      = "none"
}

resource "linode_nodebalancer_config" "http" {
  nodebalancer_id = linode_nodebalancer.data.id
  port            = 80
  protocol        = "tcp"
  algorithm       = "roundrobin"
  check           = "connection"
  check_interval  = 10
  check_timeout   = 5
  check_attempts  = 3
  stickiness      = "none"
}

resource "linode_nodebalancer_node" "data_https" {
  count = var.data_instance_count

  nodebalancer_id = linode_nodebalancer.data.id
  config_id       = linode_nodebalancer_config.https.id
  label           = "${local.prefix}-data-${count.index}-https"
  address         = "${linode_instance.data[count.index].private_ip_address}:443"
  weight          = 100
  mode            = "accept"
}

resource "linode_nodebalancer_node" "data_http" {
  count = var.data_instance_count

  nodebalancer_id = linode_nodebalancer.data.id
  config_id       = linode_nodebalancer_config.http.id
  label           = "${local.prefix}-data-${count.index}-http"
  address         = "${linode_instance.data[count.index].private_ip_address}:80"
  weight          = 100
  mode            = "accept"
}

# -----------------------------------------------------------------------------
# Firewall — NodeBalancer
# Inbound: 80/443 open (public redirect traffic)
# Default drop on all other ports
# -----------------------------------------------------------------------------

resource "linode_firewall" "nodebalancer" {
  label = "${local.prefix}-nb-fw"

  inbound {
    label    = "allow-https"
    action   = "ACCEPT"
    protocol = "TCP"
    ports    = "443"
    ipv4     = ["0.0.0.0/0"]
    ipv6     = ["::/0"]
  }

  inbound {
    label    = "allow-http"
    action   = "ACCEPT"
    protocol = "TCP"
    ports    = "80"
    ipv4     = ["0.0.0.0/0"]
    ipv6     = ["::/0"]
  }

  inbound_policy  = "DROP"
  outbound_policy = "ACCEPT"

  nodebalancers = [linode_nodebalancer.data.id]
}

# -----------------------------------------------------------------------------
# Firewall — data plane
# Inbound: NB private subnet on 80/443, admin CIDRs on 22
# -----------------------------------------------------------------------------

resource "linode_firewall" "data" {
  label = "${local.prefix}-data-fw"

  inbound {
    label    = "allow-nb-https"
    action   = "ACCEPT"
    protocol = "TCP"
    ports    = "443"
    ipv4     = ["192.168.128.0/17"]
  }

  inbound {
    label    = "allow-nb-http"
    action   = "ACCEPT"
    protocol = "TCP"
    ports    = "80"
    ipv4     = ["192.168.128.0/17"]
  }

  inbound {
    label    = "allow-admin-ssh"
    action   = "ACCEPT"
    protocol = "TCP"
    ports    = "22"
    ipv4     = var.admin_cidrs
  }

  inbound_policy  = "DROP"
  outbound_policy = "ACCEPT"

  linodes = linode_instance.data[*].id
}

# -----------------------------------------------------------------------------
# Firewall — control plane
# Inbound: admin CIDRs on 443/80/22
# -----------------------------------------------------------------------------

resource "linode_firewall" "control" {
  label = "${local.prefix}-control-fw"

  inbound {
    label    = "allow-admin-https"
    action   = "ACCEPT"
    protocol = "TCP"
    ports    = "443"
    ipv4     = var.admin_cidrs
  }

  inbound {
    label    = "allow-acme-http"
    action   = "ACCEPT"
    protocol = "TCP"
    ports    = "80"
    ipv4     = ["0.0.0.0/0"]
  }

  inbound {
    label    = "allow-admin-ssh"
    action   = "ACCEPT"
    protocol = "TCP"
    ports    = "22"
    ipv4     = var.admin_cidrs
  }

  inbound_policy  = "DROP"
  outbound_policy = "ACCEPT"

  linodes = [linode_instance.control.id]
}
