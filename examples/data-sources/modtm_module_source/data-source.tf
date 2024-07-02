data "modtm_module_source" "this" {
  module_path = path.module
}

output "module_version" {
  value = data.modtm_module_source.this.module_version
}

output "module_source" {
  value = data.modtm_module_source.this.module_source
}