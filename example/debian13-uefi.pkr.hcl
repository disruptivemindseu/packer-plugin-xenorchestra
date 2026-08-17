# For development, comment this out and use `make dev` instead
# For production, uncomment and use:
#   packer init example/debian13-uefi.pkr.hcl
#
# packer {
#   required_plugins {
#     xenorchestra = {
#       version = ">= 0.1.0"
#       source  = "github.com/disruptivemindseu/xenorchestra"
#     }
#   }
# }

variable "rest_endpoint" {
  type        = string
  description = "The URL of your Xen Orchestra. This will be pulled from the env var 'PKR_VAR_rest_endpoint'"
}
variable "token" {
  type        = string
  description = "The authentication token for Xen Orchestra. This will be pulled from the env var 'PKR_VAR_token'"
  sensitive   = true
}
// For now, username and password are needed to connect to the VM console. In the future, we will use the token to connect to the console as well.
variable "username" {
  type = string
  sensitive   = true
}
variable "password" {
  type = string
  sensitive = true
}


variable "pool_name" {
  type        = string
  description = "The name of the pool to build in. This will be pulled from the env var 'PKR_VAR_pool_name'"
}
variable "sr_iso_name" {
  type        = string
  description = "The name of the SR containing the ISO. This will be pulled from the env var 'PKR_VAR_sr_iso_name'"
}
variable "sr_name" {
  type        = string
  description = "The name of the SR to create the VM in. This will be pulled from the env var 'PKR_VAR_sr_name'"
}
variable "template_name" {
  type        = string
  description = "The name of the template to clone. This will be pulled from the env var 'PKR_VAR_template_name'"
}
variable "vm_name" {
  type        = string
  description = "The name of the VM to create. This will be pulled from the env var 'PKR_VAR_vm_name'"
  default     = null
}
variable "vm_description" {
  type        = string
  description = "The description of the VM. This will be pulled from the env var 'PKR_VAR_vm_description'"
  default     = null
}

locals {
  timestamp      = regex_replace(timestamp(), "[- TZ:]", "")
  buildtime      = formatdate("YYYY.MM.DD", timestamp())
  vm_name        = coalesce(var.vm_name, "template-debian13-uefi_${local.timestamp}")
  vm_description = coalesce(var.vm_description, "[Template] Debian 13 UEFI built on ${local.buildtime} by Packer")
}

source "xenorchestra-iso" "template" {
  rest_endpoint = var.rest_endpoint
  token         = var.token
  username = var.username
  password = var.password

  pool_name      = var.pool_name
  sr_iso_name    = var.sr_iso_name
  sr_name        = var.sr_name
  template_name  = var.template_name
  network_names  = ["DHCP - POUBELLE"]
  iso_name       = "debian-13.6.0-amd64-netinst.iso"

  vm_name        = local.vm_name
  vm_description = local.vm_description
  vcpus          = 2
  vm_memory      = 4096 * 1024 * 1024 # 4096 MB in bytes
  disk_size      = 32768
  disk_name      = "packer_disk_1"
  firmware       = "uefi"

  http_directory = "example/http"
  boot_wait      = "15s"
  mode           = "shutdown" # "tools" or "shutdown"

  boot_command = [
    "c",
    "linux /install.amd/vmlinuz ",
    "vga=788 ",
    "theme=dark ",
    "auto=true ",
    "priority=critical ",
    "url=http://{{.HTTPIP}}:{{.HTTPPort}}/preseed.cfg ",
    "hostname=template-debian13-uefi ",
    "--- ",
    "quiet ",
    "<enter>",
    "initrd /install.amd/initrd.gz ",
    "<enter>",
    "boot<enter>"
  ]
}

build {
  sources = ["xenorchestra-iso.template"]
}
