package project

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ehsanmsb/terraform-provider-healthchecks/internal/client"
	"github.com/ehsanmsb/terraform-provider-healthchecks/internal/providerdata"
)

type projectDataSource struct{ client *client.Client }

type projectLookup struct {
	ID                    types.String `tfsdk:"id"`
	Name                  types.String `tfsdk:"name"`
	APIKeyEnabled         types.Bool   `tfsdk:"api_key_enabled"`
	ReadOnlyAPIKeyEnabled types.Bool   `tfsdk:"read_only_api_key_enabled"`
	PingKeyEnabled        types.Bool   `tfsdk:"ping_key_enabled"`
}

func NewDataSource() datasource.DataSource { return &projectDataSource{} }

func (d *projectDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (d *projectDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads an existing project using the provider's username and password. Exposes the project name and whether its keys are enabled.",
		Attributes: map[string]schema.Attribute{
			"id":                        schema.StringAttribute{Required: true, MarkdownDescription: "Project UUID."},
			"name":                      schema.StringAttribute{Computed: true},
			"api_key_enabled":           schema.BoolAttribute{Computed: true},
			"read_only_api_key_enabled": schema.BoolAttribute{Computed: true},
			"ping_key_enabled":          schema.BoolAttribute{Computed: true},
		},
	}
}

func (d *projectDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	if req.ProviderData != nil {
		d.client = req.ProviderData.(*providerdata.ConfiguredClient).Client
	}
}

func (d *projectDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state projectLookup
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	project, err := d.client.GetProject(ctx, state.ID.ValueString(), "", "", "")
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read Existing Project", err.Error())
		return
	}
	state.Name = types.StringValue(project.Name)
	state.APIKeyEnabled = types.BoolValue(project.APIKeyEnabled)
	state.ReadOnlyAPIKeyEnabled = types.BoolValue(project.ReadOnlyAPIKeyEnabled)
	state.PingKeyEnabled = types.BoolValue(project.PingKeyEnabled)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
