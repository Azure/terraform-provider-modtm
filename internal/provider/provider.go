// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"os"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure ModuleTelemetryProvider satisfies various provider interfaces.
var _ provider.Provider = &ModuleTelemetryProvider{}

// ModuleTelemetryProvider defines the provider implementation.
type ModuleTelemetryProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// ModuleTelemetryProviderModel describes the provider data model.
type ModuleTelemetryProviderModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
}

func (p *ModuleTelemetryProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "modtm"
	resp.Version = p.version
}

func (p *ModuleTelemetryProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				MarkdownDescription: "Telemetry endpoint to send data to.",
				Optional:            true,
			},
		},
	}
}

func (p *ModuleTelemetryProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data ModuleTelemetryProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := "https://telemetry-proxy.purpleocean-4c6893be.eastus.azurecontainerapps.io/telemetry"
	endpointEnv := os.Getenv("MODTM_ENDPOINT")
	if !data.Endpoint.IsNull() {
		e, err := strconv.Unquote(data.Endpoint.String())
		if err != nil {
			e = data.Endpoint.String()
		}
		endpoint = e
	} else if endpointEnv != "" {
		endpoint = endpointEnv
	}
	resp.DataSourceData = endpoint
	resp.ResourceData = endpoint
}

func (p *ModuleTelemetryProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewTelemetryResource,
	}
}

func (p *ModuleTelemetryProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return nil
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &ModuleTelemetryProvider{
			version: version,
		}
	}
}
