package integration

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ehsanmsb/terraform-provider-healthchecks/internal/client"
	"github.com/ehsanmsb/terraform-provider-healthchecks/internal/providerdata"
)

type integrationDataSource struct{ client *client.Client }

type integrationLookup struct {
	ID            types.String `tfsdk:"id"`
	ProjectID     types.String `tfsdk:"project_id"`
	ProjectAPIKey types.String `tfsdk:"project_api_key"`
	Type          types.String `tfsdk:"type"`
	Name          types.String `tfsdk:"name"`
}

func NewDataSource() datasource.DataSource { return &integrationDataSource{} }

func (d *integrationDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration"
}

func (d *integrationDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up an existing webhook or email integration through the project Management API.",
		Attributes: map[string]schema.Attribute{
			"id":         schema.StringAttribute{Required: true, MarkdownDescription: "Channel UUID from Healthchecks. Use it in a check's channels list."},
			"project_id": schema.StringAttribute{Required: true, MarkdownDescription: "ID of the project associated with the read-write API key."},
			"type":       schema.StringAttribute{Required: true, MarkdownDescription: "Integration type: `webhook` or `email`."},
			"project_api_key": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Read-write key for the project. Omit when the provider is configured with that project's API key.",
			},
			"name": schema.StringAttribute{Computed: true, MarkdownDescription: "Channel name returned by Healthchecks. May be empty."},
		},
	}
}

func (d *integrationDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	if req.ProviderData != nil {
		d.client = req.ProviderData.(*providerdata.ConfiguredClient).Client
	}
}

func (d *integrationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state integrationLookup
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	kind := state.Type.ValueString()
	if kind != "webhook" && kind != "email" {
		resp.Diagnostics.AddError("Unsupported Integration Type", "Set type to webhook or email.")
		return
	}
	channels, err := d.client.ListChannels(ctx, state.ProjectID.ValueString(), state.ProjectAPIKey.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to List Project Integrations", err.Error())
		return
	}
	for _, channel := range channels {
		if channel.ID == state.ID.ValueString() && channel.Kind == kind {
			state.Name = types.StringValue(channel.Name)
			resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
			return
		}
	}
	resp.Diagnostics.AddError("Integration Not Found", fmt.Sprintf("No %s channel with ID %q was found for the supplied project API key.", kind, state.ID.ValueString()))
}
