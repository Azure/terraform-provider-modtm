package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

var _ validator.Map = mapValidator{}

type mapValidator struct{}

func (m mapValidator) Description(ctx context.Context) string {
	return "`tags` could not contains key `event`."
}

func (m mapValidator) MarkdownDescription(ctx context.Context) string {
	return "`tags` could not contains key `event`."
}

func (m mapValidator) ValidateMap(ctx context.Context, request validator.MapRequest, response *validator.MapResponse) {
	for k := range request.ConfigValue.Elements() {
		if k == "event" {
			response.Diagnostics.AddError("`tags` could not contains key `event`.", "")
		}
	}
}
