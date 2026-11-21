package check

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ehsanmsb/terraform-provider-healthchecks/internal/client"
	"github.com/ehsanmsb/terraform-provider-healthchecks/internal/providerdata"
)

var _ datasource.DataSourceWithConfigure = (*checkDataSource)(nil)

type checkDataSource struct{ client *client.Client }

func NewDataSource() datasource.DataSource { return &checkDataSource{} }

func (d *checkDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_check"
}

func (d *checkDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads an existing Healthchecks check by UUID through the Management API v3.",
		Attributes: map[string]schema.Attribute{
			"id":         schema.StringAttribute{Required: true, MarkdownDescription: "UUID of the existing check."},
			"project_id": schema.StringAttribute{Required: true, MarkdownDescription: "ID of the project that owns the check."},
			"project_api_key": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Read-write key for the project. It can be omitted when the provider is configured with the project's API key.",
			},
			"name":     schema.StringAttribute{Computed: true},
			"slug":     schema.StringAttribute{Computed: true},
			"tags":     schema.ListAttribute{Computed: true, ElementType: types.StringType},
			"desc":     schema.StringAttribute{Computed: true},
			"timeout":  schema.Int64Attribute{Computed: true},
			"grace":    schema.Int64Attribute{Computed: true},
			"schedule": schema.StringAttribute{Computed: true},
			"tz":       schema.StringAttribute{Computed: true},
			"channels": schema.ListAttribute{Computed: true, ElementType: types.StringType},
			"uuid":     schema.StringAttribute{Computed: true},
			"ping_url": schema.StringAttribute{Computed: true, Sensitive: true},
			"status":   schema.StringAttribute{Computed: true},
		},
	}
}

func (d *checkDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	if req.ProviderData != nil {
		d.client = req.ProviderData.(*providerdata.ConfiguredClient).Client
	}
}

func (d *checkDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state model
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	check, err := d.client.GetCheck(ctx, state.ProjectID.ValueString(), state.ProjectAPIKey.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read Existing Check", err.Error())
		return
	}

	applyCheckToState(&state, check)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
