package provider

import (
	"context"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/helpers/validatordiag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

type MustBeValidRegex struct {
}

func (m MustBeValidRegex) Description(ctx context.Context) string {
	return "value must be a valid regex"
}

func (m MustBeValidRegex) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m MustBeValidRegex) ValidateString(ctx context.Context, request validator.StringRequest, response *validator.StringResponse) {
	item := request.ConfigValue.ValueString()
	_, err := regexp.Compile(item)
	if err != nil {
		response.Diagnostics.Append(validatordiag.InvalidAttributeValueDiagnostic(request.Path, m.Description(ctx), item))
	}
}
