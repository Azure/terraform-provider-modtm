// Copyright (c) Microsoft Corporation. All rights reserved.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

type moduleSource interface {
	GetModuleVersion() types.String
	SetModuleVersion(types.String)
	GetModuleSource() types.String
	SetModuleSource(types.String)
	GetModulePath() types.String
}

// withModuleSourceAndVersion updates the module source and version based on the module path.
func withModuleSourceAndVersion[T moduleSource](data T) T {
	data.SetModuleSource(basetypes.NewStringNull())
	data.SetModuleVersion(basetypes.NewStringNull())
	if !data.GetModulePath().IsNull() && !data.GetModulePath().IsUnknown() {
		module, err := parseModulesJson(data.GetModulePath().ValueString())
		if err != nil {
			return data
		}
		data.SetModuleSource(types.StringValue(module.Source))
		data.SetModuleVersion(types.StringValue(module.Version))
	}
	return data
}
