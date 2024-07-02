// Copyright (c) Microsoft Corporation. All rights reserved.
// SPDX-License-Identifier: MPL-2.0

package provider

import "github.com/hashicorp/terraform-plugin-framework/types"

type moduleSource interface {
	GetModuleVersion() types.String
	SetModuleVersion(types.String)
	GetModuleSource() types.String
	SetModuleSource(types.String)
	GetModulePath() types.String
}
