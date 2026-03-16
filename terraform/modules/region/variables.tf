variable "region" {
  description = "Linode region slug (e.g. us-ord, us-iad)"
  type        = string
}

variable "region_label" {
  description = "Short human-readable region label used in resource names (e.g. ord, iad)"
  type        = string
}

variable "ssh_public_key_path" {
  description = "Path to SSH public key for instance access"
  type        = string
}

variable "admin_cidrs" {
  description = "List of CIDR blocks allowed SSH and direct HTTP(S) access to control plane and SSH to data plane"
  type        = list(string)
}

variable "data_instance_type" {
  description = "Linode instance type for data plane nodes"
  type        = string
  default     = "g6-dedicated-2"
}

variable "control_instance_type" {
  description = "Linode instance type for control plane node"
  type        = string
  default     = "g6-dedicated-2"
}

variable "data_instance_count" {
  description = "Number of data plane instances behind the NodeBalancer"
  type        = number
  default     = 2
}

variable "pg_engine_type" {
  description = "Linode Managed PostgreSQL engine type"
  type        = string
  default     = "g6-nanode-1"
}

variable "pg_engine_id" {
  description = "PostgreSQL engine version identifier"
  type        = string
  default     = "postgresql/16"
}

variable "pg_cluster_size" {
  description = "Number of nodes in the PostgreSQL DBaaS cluster"
  type        = number
  default     = 3
}

variable "tags" {
  description = "Tags applied to all resources in this region"
  type        = list(string)
  default     = ["project:tld-redirect", "customer:tld-demo", "owner:brian"]
}
