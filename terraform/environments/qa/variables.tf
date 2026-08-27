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
  description = "Additional CIDRs allowed to the control plane admin UI on :443 only (PNC Zscaler egress)"
  type        = list(string)
  default     = ["170.85.8.0/24", "136.226.52.0/24"]
}

variable "tags" {
  description = "Tags applied to all resources"
  type        = list(string)
  default     = ["project:pnc-redirect", "customer:pnc", "tier:qa", "owner:brian"]
}
