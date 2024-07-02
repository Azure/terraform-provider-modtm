package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ function.Function = &ModuleSourceFunction{}

func NewModuleSourceFunction() function.Function {
	return &ModuleSourceFunction{}
}

type ModuleSourceFunction struct {
}

func (m *ModuleSourceFunction) Metadata(ctx context.Context, req function.MetadataRequest, resp *function.MetadataResponse) {
	resp.Name = "module_source"
}

func (m *ModuleSourceFunction) Definition(ctx context.Context, req function.DefinitionRequest, resp *function.DefinitionResponse) {
	resp.Definition = function.Definition{
		Summary:             "`module_source` function",
		MarkdownDescription: "This function takes in `${path.module}` and return the corresponding item's `Source` in `modules.json` file in the current root module's `.terraform/module` folder",
		Parameters: []function.Parameter{
			function.StringParameter{
				Name:                "module_path",
				MarkdownDescription: "`${path.module}`",
			},
		},
		Return: function.StringReturn{},
	}
}

func (m *ModuleSourceFunction) Run(ctx context.Context, req function.RunRequest, resp *function.RunResponse) {
	var modulePath string
	resp.Error = function.ConcatFuncErrors(req.Arguments.Get(ctx, &modulePath))
	if resp.Error != nil {
		return
	}
	model := &ModuleSourceDataSourceModel{}
	model.ModulePath = types.StringValue(modulePath)
	model = withModuleSourceAndVersion(model)
	s := model.ModuleSource.ValueString()
	resp.Error = function.ConcatFuncErrors(resp.Result.Set(ctx, s))
}
