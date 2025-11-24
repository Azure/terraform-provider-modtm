// Copyright (c) Microsoft Corporation. All rights reserved.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"sync"
	"time"

	listvalidators "github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

var defaultEndpointUrl = "https://aka.ms/avmtelemetrysvc/telemetry/20251119"

// Ensure ModuleTelemetryProvider satisfies various provider interfaces.
var _ provider.Provider = &ModuleTelemetryProvider{}

// ModuleTelemetryProvider defines the provider implementation.
type ModuleTelemetryProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version            string
	useDefaultEndpoint bool
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

	c := providerConfig{
		endpointFunc: func() string {
			once.Do(func() {
				initialEndpoint := p.readEndpoint(data, ctx)
				endpoint = checkAndFollowRedirect(initialEndpoint)
				if endpoint != initialEndpoint {
					traceLog(ctx, fmt.Sprintf("Endpoint redirected (301) to: %s", endpoint))
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

	c.defaultEndpoint = p.useDefaultEndpoint
	resp.DataSourceData = c
	resp.ResourceData = resp.DataSourceData
}

func (p *ModuleTelemetryProvider) readEndpoint(data ModuleTelemetryProviderModel, ctx context.Context) string {
	var endpoint string
	if !data.Endpoint.IsNull() {
		endpoint = readEndpointFromProviderBlock(data)
		traceLog(ctx, fmt.Sprintf("Load provider's endpoint from provider block: %s", endpoint))
	} else if endpointEnv := os.Getenv("MODTM_ENDPOINT"); endpointEnv != "" {
		endpoint = endpointEnv
		traceLog(ctx, fmt.Sprintf("Load provider's endpoint from environment variable: %s", endpoint))
	} else {
		endpoint = defaultEndpointUrl
		p.useDefaultEndpoint = true
		traceLog(ctx, fmt.Sprintf("Load provider's endpoint from default URL: %s", endpoint))
	}
	return endpoint
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

var readDefaultEndpointTimeout = 10 * time.Second

func checkAndFollowRedirect(endpoint string) string {
	deadline := time.Now().Add(readDefaultEndpointTimeout)
	return checkAndFollowRedirectWithDeadline(endpoint, 0, 10, deadline)
}

func checkAndFollowRedirectWithDeadline(endpoint string, depth int, maxDepth int, deadline time.Time) string {
	if endpoint == "" || depth >= maxDepth {
		return endpoint
	}

	timeout := time.Until(deadline)
	if timeout <= 0 {
		return endpoint
	}

	c := make(chan string)
	go func() {
		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
			Timeout: timeout,
		}
		resp, err := client.Get(endpoint) // #nosec G107
		if err != nil {
			c <- endpoint
			return
		}
		defer func() {
			_ = resp.Body.Close()
		}()

		if resp.StatusCode == 301 {
			location := resp.Header.Get("Location")
			if location != "" && location != endpoint {
				c <- checkAndFollowRedirectWithDeadline(location, depth+1, maxDepth, deadline)
				return
			}
		}
		c <- endpoint
	}()
	select {
	case result := <-c:
		return result
	case <-time.After(timeout):
		return endpoint
	}
}
