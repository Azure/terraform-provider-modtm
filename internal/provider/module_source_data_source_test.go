// Copyright (c) Microsoft Corporation. All rights reserved.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAccModuleSourceDataSource(t *testing.T) {
	require.NoError(t, createModulesJson())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccModuleSourceDataSourceConfig(".terraform/modules/kv"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.modtm_module_source.test", "module_path", ".terraform/modules/kv"),
					resource.TestCheckResourceAttr("data.modtm_module_source.test", "module_source", "registry.terraform.io/Azure/avm-res-keyvault-vault/azurerm"),
					resource.TestCheckResourceAttr("data.modtm_module_source.test", "module_version", "0.6.1"),
				),
			},
			{
				Config: testAccModuleSourceDataSourceConfig(".terraform/modules/kv/modules/key"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.modtm_module_source.test", "module_path", ".terraform/modules/kv/modules/key"),
					resource.TestCheckResourceAttr("data.modtm_module_source.test", "module_source", "./modules/key"),
					resource.TestCheckResourceAttr("data.modtm_module_source.test", "module_version", ""),
				),
			},
		},
	})
}

func testAccModuleSourceDataSourceConfig(modulePath string) string {
	return fmt.Sprintf(`
provider "modtm" {
  enabled = false
}

data "modtm_module_source" "test" {
  module_path = "%s"
}
`, modulePath)
}
