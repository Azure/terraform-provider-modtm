// Copyright (c) Microsoft Corporation. All rights reserved.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/require"
	"os"
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
			{
				Config: testAccModuleSourceDataSourceConfig(".terraform/modules/nonexistent"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.modtm_module_source.test", "module_path", ".terraform/modules/nonexistent"),
					resource.TestCheckNoResourceAttr("data.modtm_module_source.test", "module_source"),
					resource.TestCheckNoResourceAttr("data.modtm_module_source.test", "module_version"),
				),
			},
			{
				Config: testAccModuleSourceDataSourceConfig(".terraform/modules/keys/modules/key"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.modtm_module_source.test", "module_path", ".terraform/modules/keys/modules/key"),
					resource.TestCheckResourceAttr("data.modtm_module_source.test", "module_source", "registry.terraform.io/Azure/avm-res-keyvault-vault/azurerm//modules/key"),
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

// createModulesJson creates a modules.json file with a reference to a standard module (kv) and
// reference to a child module of the key vault module (keys) in the root module.
func createModulesJson() error {
	if err := os.MkdirAll(".terraform/modules", 0755); err != nil {
		return err
	}
	f, err := os.Create(".terraform/modules/modules.json")
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(`{
  "Modules": [
    {
      "Key": "",
      "Source": "",
      "Dir": "."
    },
    {
      "Key": "keys",
      "Source": "registry.terraform.io/Azure/avm-res-keyvault-vault/azurerm//modules/key",
      "Version": "0.6.1",
      "Dir": ".terraform/modules/keys/modules/key"
    },
    {
      "Key": "kv",
      "Source": "registry.terraform.io/Azure/avm-res-keyvault-vault/azurerm",
      "Version": "0.6.1",
      "Dir": ".terraform/modules/kv"
    },
    {
      "Key": "kv.keys",
      "Source": "./modules/key",
      "Dir": ".terraform/modules/kv/modules/key"
    },
    {
      "Key": "kv.secrets",
      "Source": "./modules/secret",
      "Dir": ".terraform/modules/kv/modules/secret"
    }
  ]
}
`)
	return err
}
