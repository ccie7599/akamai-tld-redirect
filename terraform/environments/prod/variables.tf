# -----------------------------------------------------------------------------
# Production environment variables
# -----------------------------------------------------------------------------

# --- Infrastructure ---

variable "ssh_public_key_path" {
  description = "Path to SSH public key for instance access"
  type        = string
  default     = "~/.ssh/id_rsa.pub"
}

variable "admin_cidrs" {
  description = "CIDR blocks allowed direct access (SSH, control plane HTTP/S)"
  type        = list(string)
}

variable "ui_cidrs" {
  description = "Additional CIDRs allowed to the control plane admin UI on :443 only (not SSH). PNC Zscaler egress ZENs — INTERIM until PNC provides a dedicated egress IP; a /24 admits all Zscaler customers on that ZEN."
  type        = list(string)
  default     = ["170.85.8.0/24", "136.226.52.0/24"]
}

variable "data_instance_type" {
  description = "Linode instance type for data plane nodes"
  type        = string
  default     = "g6-dedicated-2"
}

variable "control_instance_type" {
  description = "Linode instance type for control plane nodes"
  type        = string
  default     = "g6-dedicated-2"
}

variable "tags" {
  description = "Tags applied to all resources"
  type        = list(string)
  default     = ["project:pnc-redirect", "customer:pnc", "tier:delivery", "owner:brian"]
}

# --- Akamai / DS2 ---

variable "ds2_stream_id" {
  description = "DataStream 2 stream ID (0 = not yet created, auto-populated after first apply)"
  type        = number
  default     = 0
}

variable "ds2_webhook_host" {
  description = "DS2 webhook receiver hostname (existing ds2-ingest service)"
  type        = string
  default     = "ds2-im-demo.connected-cloud.io"
}

variable "ds2_webhook_username" {
  description = "DS2 webhook HTTP Basic Auth username"
  type        = string
  sensitive   = true
  default     = ""
}

variable "ds2_webhook_password" {
  description = "DS2 webhook HTTP Basic Auth password"
  type        = string
  sensitive   = true
  default     = ""
}
