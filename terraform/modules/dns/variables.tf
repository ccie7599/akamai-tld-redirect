variable "zone" {
  description = "Akamai Edge DNS zone"
  type        = string
  default     = "connected-cloud.io"
}

variable "a_records" {
  description = "Map of FQDN -> list of IPv4 addresses for A records"
  type        = map(list(string))
}

variable "ttl" {
  description = "TTL in seconds for all A records"
  type        = number
  default     = 300
}
