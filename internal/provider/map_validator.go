// Copyright (c) Microsoft Corporation. All rights reserved.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"slices"

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
	reservedKeys := []string{"event"}
	for k := range request.ConfigValue.Elements() {
		if slices.Contains(reservedKeys, k) {
			errStr := fmt.Sprintf("`tags` must not contains keys %v.", reservedKeys)
			response.Diagnostics.AddError(errStr, "")
		}
	}
}
