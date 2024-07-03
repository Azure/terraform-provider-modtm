resource "modtm_telemetry" "test" {
  tags = {
    avm_git_file       = "main.tf"
    avm_module_source  = provider::modtm::module_source(path.module)
    avm_module_version = provider::modtm::module_version(path.module)
  }
}