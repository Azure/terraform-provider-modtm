// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"net/http"
	"strconv"
	"time"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &TelemetryResource{}
var _ resource.ResourceWithImportState = &TelemetryResource{}

var traceLog = tflog.Trace
var errorLog = tflog.Error

func NewTelemetryResource() resource.Resource {
	return &TelemetryResource{}
}

// TelemetryResource defines the resource implementation.
type TelemetryResource struct {
	endpoint string
	enabled  bool
}

// TelemetryResourceModel describes the resource data model.
type TelemetryResourceModel struct {
	Id   types.String `tfsdk:"id"`
	Tags types.Map    `tfsdk:"tags"`
}

func (r *TelemetryResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_telemetry"
}

func (r *TelemetryResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "`modtm_telemetry` resource gathers and sends telemetry data to a specified endpoint. The aim is to provide visibility into the lifecycle of your Terraform modules - whether they are being created, updated, or deleted.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Resource identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"tags": schema.MapAttribute{
				Required:    true,
				ElementType: basetypes.StringType{},
				Validators: []validator.Map{
					mapValidator{},
				},
			},
		},
	}
}

func (r *TelemetryResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	config, ok := req.ProviderData.(providerConfig)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected providerConfig, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.endpoint = config.endpoint
	r.enabled = config.enabled
}

func (r *TelemetryResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data TelemetryResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	newId := uuid.NewString()
	data.Id = types.StringValue(newId)
	traceLog(ctx, fmt.Sprintf("created telemetry resource with id %s", newId))
	data.sendTags(ctx, r, "create")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TelemetryResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data TelemetryResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	traceLog(ctx, fmt.Sprintf("read telemetry resource with id %s", data.Id.String()))
	data.sendTags(ctx, r, "read")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TelemetryResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data TelemetryResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	traceLog(ctx, fmt.Sprintf("update telemetry resource with id %s", data.Id.String()))
	data.sendTags(ctx, r, "update")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TelemetryResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data TelemetryResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	traceLog(ctx, fmt.Sprintf("delete telemetry resource with id %s", data.Id.String()))
	data.sendTags(ctx, r, "delete")
}

func (r *TelemetryResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// sendPostRequest sends an HTTP POST request to the specified URL with the given body.
func sendPostRequest(ctx context.Context, url string, tags map[string]string) {
	jsonStr, err := json.Marshal(tags)
	if err != nil {
		tflog.Error(ctx, fmt.Sprintf("error on unmarshal telemetry resource: %s", err.Error()))
		return
	}
	event := tags["event"]
	client := &http.Client{}
	traceLog(ctx, fmt.Sprintf("sending tags to %s", url))
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
	if err != nil {
		errorLog(ctx, fmt.Sprintf("error on composing http request for %s telemetry resource: %s", event, err.Error()))
	}
	req.Header.Set("Content-Type", "application/json")
	c := make(chan int)
	go func() {
		defer close(c)
		resp, err := client.Do(req)
		if err != nil {
			errorLog(ctx, fmt.Sprintf("error on %s telemetry resource: %s", event, err.Error()))
		}
		traceLog(ctx, fmt.Sprintf("response Status for %s telemetry resource: %s", event, resp.Status))
		defer resp.Body.Close()
		c <- 1
	}()
	select {
	case <-c:
		return
	case <-time.After(5 * time.Second):
		errorLog(ctx, fmt.Sprintf("timeout on %s telemetry resource", event))
		return
	}
}

func (data TelemetryResourceModel) sendTags(ctx context.Context, r *TelemetryResource, event string) {
	if !r.enabled {
		return
	}
	tags := make(map[string]string)
	for k, v := range data.Tags.Elements() {
		value, err := strconv.Unquote(v.String())
		if err != nil {
			value = v.String()
		}
		tags[k] = value
	}
	tags["event"] = event
	resourceId, err := strconv.Unquote(data.Id.String())
	if err != nil {
		resourceId = data.Id.String()
	}
	tags["resource_id"] = resourceId
	sendPostRequest(ctx, r.endpoint, tags)
}
