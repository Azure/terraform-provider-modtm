package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ function.Function = &ModuleVersionFunction{}

func NewModuleVersionFunction() function.Function {
	return &ModuleVersionFunction{}
}

type ModuleVersionFunction struct {
}

func (m *ModuleVersionFunction) Metadata(ctx context.Context, req function.MetadataRequest, resp *function.MetadataResponse) {
	resp.Name = "module_version"
}

func (m *ModuleVersionFunction) Definition(ctx context.Context, req function.DefinitionRequest, resp *function.DefinitionResponse) {
	resp.Definition = function.Definition{
		Summary:             "`module_source` function",
		MarkdownDescription: "This function takes in `${path.module}` and return the corresponding item's `Version` in `modules.json` file in the current root module's `.terraform/module` folder",
		Parameters: []function.Parameter{
			function.StringParameter{
				Name:                "module_path",
				MarkdownDescription: "`${path.module}`",
			},
		},
		Return: function.StringReturn{},
	}
}

func (m *ModuleVersionFunction) Run(ctx context.Context, req function.RunRequest, resp *function.RunResponse) {
	var modulePath string
	resp.Error = function.ConcatFuncErrors(req.Arguments.Get(ctx, &modulePath))
	if resp.Error != nil {
		return
	}
	model := &ModuleSourceDataSourceModel{}
	model.ModulePath = types.StringValue(modulePath)
	model = withModuleSourceAndVersion(model)
	s := model.ModuleVersion.ValueString()
	resp.Error = function.ConcatFuncErrors(resp.Result.Set(ctx, s))
}
