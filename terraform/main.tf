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

resource "linode_instance" "redirect" {
  label           = "tld-redirect"
  region          = var.region
  type            = var.instance_type
  image           = "linode/ubuntu24.04"
  authorized_keys = [trimspace(file(var.ssh_public_key_path))]
  tags            = ["project:tld-redirect", "customer:tld-demo", "owner:brian"]

  metadata {
    user_data = base64encode(templatefile("${path.module}/cloud-init.yaml", {}))
  }
}

resource "linode_firewall" "redirect" {
  label = "tld-redirect-fw"

  inbound {
    label    = "allow-http"
    action   = "ACCEPT"
    protocol = "TCP"
    ports    = "80"
    ipv4     = ["0.0.0.0/0"]
    ipv6     = ["::/0"]
  }

  inbound {
    label    = "allow-https"
    action   = "ACCEPT"
    protocol = "TCP"
    ports    = "443"
    ipv4     = ["0.0.0.0/0"]
    ipv6     = ["::/0"]
  }

  inbound {
    label    = "allow-ssh"
    action   = "ACCEPT"
    protocol = "TCP"
    ports    = "22"
    ipv4     = ["0.0.0.0/0"]
    ipv6     = ["::/0"]
  }

  inbound_policy  = "DROP"
  outbound_policy = "ACCEPT"

  linodes = [linode_instance.redirect.id]
}
