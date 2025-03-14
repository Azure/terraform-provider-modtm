// Copyright (c) Microsoft Corporation. All rights reserved.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	listvalidators "github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
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
	Endpoint          types.String `tfsdk:"endpoint"`
	Enabled           types.Bool   `tfsdk:"enabled"`
	ModuleSourceRegex types.List   `tfsdk:"module_source_regex"`
}

type providerConfig struct {
	endpointFunc      func() string
	enabled           bool
	defaultEndpoint   bool
	moduleSourceRegex []*regexp.Regexp
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
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Sending telemetry or not, set this argument to `false` would turn telemetry off. Defaults to `true`.",
				Optional:            true,
			},
			"module_source_regex": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				MarkdownDescription: "List of regex as allow list for module source. Only module source that match one of the regex will be collected.",
				Validators: []validator.List{
					listvalidators.SizeAtLeast(1),
					listvalidators.ValueStringsAre(&MustBeValidRegex{}),
				},
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
	enabled := true
	if !data.Enabled.IsNull() {
		enabled = data.Enabled.ValueBool()
	}
	var once sync.Once
	endpoint := ""
	endpointEnv := os.Getenv("MODTM_ENDPOINT")

	c := providerConfig{
		endpointFunc: func() string {
			once.Do(func() {
				if !data.Endpoint.IsNull() {
					endpoint = readEndpointFromProviderBlock(data)
					traceLog(ctx, fmt.Sprintf("Load provider's endpoint from provider block: %s", endpoint))
				} else if endpointEnv != "" {
					endpoint = endpointEnv
					traceLog(ctx, fmt.Sprintf("Load provider's endpoint from environment variable: %s", endpoint))
				} else {
					e, err := readEndpointFromBlob()
					if err != nil {
						endpoint = ""
						traceLog(ctx, "Failed to load provider's endpoint from default blob storage")
						return
					}
					endpoint = e
					traceLog(ctx, fmt.Sprintf("Load provider's endpoint from default blob storage: %s", endpoint))
				}

			})
			return endpoint
		},
		enabled: enabled,
	}

	if !data.ModuleSourceRegex.IsNull() {
		for _, value := range data.ModuleSourceRegex.Elements() {
			c.moduleSourceRegex = append(c.moduleSourceRegex, regexp.MustCompile(value.(basetypes.StringValue).ValueString()))
		}
	}
	if len(c.moduleSourceRegex) == 0 {
		c.moduleSourceRegex = append(c.moduleSourceRegex, regexp.MustCompile(".*"))
	}

	c.defaultEndpoint = data.Endpoint.IsNull() && endpointEnv == ""
	resp.DataSourceData = c
	resp.ResourceData = resp.DataSourceData
}

func readEndpointFromProviderBlock(data ModuleTelemetryProviderModel) string {
	e, err := strconv.Unquote(data.Endpoint.String())
	if err != nil {
		return data.Endpoint.String()
	}
	return e
}

func (p *ModuleTelemetryProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewTelemetryResource,
	}
}

func (p *ModuleTelemetryProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewModuleSourceDataSource,
	}
}

func (p *ModuleTelemetryProvider) Functions(ctx context.Context) []func() function.Function {
	return []func() function.Function{
		NewModuleSourceFunction,
		NewModuleVersionFunction,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &ModuleTelemetryProvider{
			version: version,
		}
	}
}

var endpointBlobUrl = "https://avmtftelemetrysvc.blob.core.windows.net/blob/endpoint"

func readEndpointFromBlob() (string, error) {
	c := make(chan int)
	errChan := make(chan error)
	var endpoint string
	var returnError error
	go func() {
		resp, err := http.Get(endpointBlobUrl) // #nosec G107
		if err != nil {
			errChan <- err
			return
		}
		defer func() {
			_ = resp.Body.Close()
		}()

		bytes, err := io.ReadAll(resp.Body)
		if err != nil {
			errChan <- err
			return
		}
		endpoint = string(bytes)
		c <- 1
	}()
	select {
	case <-c:
		return endpoint, returnError
	case err := <-errChan:
		return "", err
	case <-time.After(5 * time.Second):
		return "", fmt.Errorf("timeout on reading default endpoint")
	}
}
