package integration

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ehsanmsb/terraform-provider-healthchecks/internal/client"
	"github.com/ehsanmsb/terraform-provider-healthchecks/internal/providerdata"
)

var (
	_ resource.Resource                = (*integrationResource)(nil)
	_ resource.ResourceWithImportState = (*integrationResource)(nil)
)

func New() resource.Resource { return &integrationResource{} }

type integrationResource struct{ client *client.Client }

type model struct {
	ID        types.String `tfsdk:"id"`
	ProjectID types.String `tfsdk:"project_id"`
	Type      types.String `tfsdk:"type"`
	Name      types.String `tfsdk:"name"`
	Config    types.Map    `tfsdk:"config"`
}

func (r *integrationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration"
}

func (r *integrationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages project integrations. The initial implementation supports webhook channels through authenticated web form endpoints (`/projects/<code>/add_webhook/`, `/integrations/<code>/edit/`, `/integrations/<code>/remove/`).",
		Attributes: map[string]schema.Attribute{
			"id":         schema.StringAttribute{Computed: true},
			"project_id": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"type":       schema.StringAttribute{Required: true},
			"name":       schema.StringAttribute{Optional: true},
			"config":     schema.MapAttribute{Required: true, ElementType: types.StringType},
		},
	}
}

func (r *integrationResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*providerdata.ConfiguredClient).Client
}

func (r *integrationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if plan.Type.ValueString() != "webhook" {
		resp.Diagnostics.AddError("Unsupported Integration Type", "Only `webhook` is currently implemented.")
		return
	}
	cfg, _ := mapToStrings(ctx, plan.Config)
	integration, err := r.client.CreateWebhookIntegration(ctx, client.Integration{
		ProjectID: plan.ProjectID.ValueString(),
		Type:      "webhook",
		Name:      plan.Name.ValueString(),
		Config:    cfg,
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to Create Integration", err.Error())
		return
	}
	state := plan
	applyIntegration(&state, integration)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *integrationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	integration, err := r.client.GetWebhookIntegration(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil {
		if errors.Is(err, client.ErrNotFound()) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to Read Integration", err.Error())
		return
	}
	applyIntegration(&state, integration)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *integrationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	cfg, _ := mapToStrings(ctx, plan.Config)
	integration, err := r.client.UpdateWebhookIntegration(ctx, client.Integration{
		ID:        plan.ID.ValueString(),
		ProjectID: plan.ProjectID.ValueString(),
		Type:      plan.Type.ValueString(),
		Name:      plan.Name.ValueString(),
		Config:    cfg,
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to Update Integration", err.Error())
		return
	}
	state := plan
	applyIntegration(&state, integration)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *integrationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteIntegration(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to Delete Integration", err.Error())
	}
}

func (r *integrationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid Import ID", "Use `project_id/channel_id`.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("type"), "webhook")...)
}

func mapToStrings(ctx context.Context, m types.Map) (map[string]string, error) {
	out := map[string]string{}
	if m.IsNull() {
		return out, nil
	}
	diags := m.ElementsAs(ctx, &out, false)
	if diags.HasError() {
		return nil, errors.New(diags.Errors()[0].Summary())
	}
	return out, nil
}

func applyIntegration(state *model, in *client.Integration) {
	state.ID = types.StringValue(in.ID)
	state.ProjectID = types.StringValue(in.ProjectID)
	state.Type = types.StringValue(in.Type)
	state.Name = types.StringValue(in.Name)

	keys := make([]string, 0, len(in.Config))
	for key := range in.Config {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	values := map[string]attr.Value{}
	for _, key := range keys {
		values[key] = types.StringValue(in.Config[key])
	}
	state.Config = types.MapValueMust(types.StringType, values)
}
