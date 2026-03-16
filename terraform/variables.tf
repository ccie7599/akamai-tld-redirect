variable "region" {
  description = "Linode region"
  type        = string
  default     = "us-ord"
}

variable "instance_type" {
  description = "Linode instance type"
  type        = string
  default     = "g6-dedicated-2"
}

variable "ssh_public_key_path" {
  description = "Path to SSH public key for instance access"
  type        = string
  default     = "~/.ssh/id_rsa.pub"
}

# --- DS2 / Akamai ---

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
