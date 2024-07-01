// Copyright (c) Microsoft Corporation. All rights reserved.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &ModuleSourceDataSource{}

type ModuleSourceDataSource struct{}

func NewModuleSourceDataSource() datasource.DataSource {
	return &ModuleSourceDataSource{}
}

var _ moduleSource = &ModuleSourceDataSourceModel{}

type ModuleSourceDataSourceModel struct {
	ModulePath    types.String `tfsdk:"module_path"`
	ModuleVersion types.String `tfsdk:"module_version"`
	ModuleSource  types.String `tfsdk:"module_source"`
}

func (m *ModuleSourceDataSourceModel) GetModuleVersion() types.String {
	return m.ModuleVersion
}

func (m *ModuleSourceDataSourceModel) SetModuleVersion(v types.String) {
	m.ModuleVersion = v
}

func (m *ModuleSourceDataSourceModel) GetModuleSource() types.String {
	return m.ModuleSource
}

func (m *ModuleSourceDataSourceModel) SetModuleSource(v types.String) {
	m.ModuleSource = v
}

func (m *ModuleSourceDataSourceModel) GetModulePath() types.String {
	return m.ModulePath
}

func (m *ModuleSourceDataSource) Metadata(ctx context.Context, request datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_module_source"
}

func (m *ModuleSourceDataSource) Schema(ctx context.Context, request datasource.SchemaRequest, response *datasource.SchemaResponse) {
	response.Schema = schema.Schema{
		MarkdownDescription: "`modtm_module_source` data source is used to read the source and version that the current module is associated with. It tried to read `modules.json` file in `.terraform/modules` folder during the plan time.",
		Attributes: map[string]schema.Attribute{
			"module_path": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The path of the module that the telemetry resource is associated with. From this data the provider will attempt to read the `$TF_DATA_DIR/modules/modules.json` file and will send the module source and version to the telemetry endpoint.",
			},
			"module_version": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The version of the module that the telemetry resource is associated with",
			},
			"module_source": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The source of the module that the telemetry resource is associated with",
			},
		},
	}
}

func (m *ModuleSourceDataSource) Read(ctx context.Context, request datasource.ReadRequest, response *datasource.ReadResponse) {
	data := &ModuleSourceDataSourceModel{}
	response.Diagnostics.Append(request.Config.Get(ctx, data)...)

	if response.Diagnostics.HasError() {
		return
	}

	data = withModuleSourceAndVersion(data)
	traceLog(ctx, fmt.Sprintf("read module source for path %s, source: %s, version: %s", data.ModulePath.String(), data.ModuleSource.String(), data.ModuleVersion.String()))
	response.Diagnostics.Append(response.State.Set(ctx, data)...)
}
