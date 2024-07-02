package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/stretchr/testify/require"
)

func TestAccModuleSourceFunction(t *testing.T) {
	require.NoError(t, createModulesJson())

	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccModuleSourceFunctionConfig(".terraform/modules/keys/modules/key"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("test", "registry.terraform.io/Azure/avm-res-keyvault-vault/azurerm//modules/key"),
				),
			},
			{
				Config: testAccModuleSourceFunctionConfig(".terraform/modules/kv/modules/key"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("test", "./modules/key"),
				),
			},
		},
	})
}

func testAccModuleSourceFunctionConfig(modulePath string) string {
	return fmt.Sprintf(`
output "test" {
  value = provider::modtm::module_source("%s")
}
`, modulePath)
}
