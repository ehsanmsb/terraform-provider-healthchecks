package project

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ehsanmsb/terraform-provider-healthchecks/internal/client"
	"github.com/ehsanmsb/terraform-provider-healthchecks/internal/providerdata"
)

var (
	_ resource.Resource                = (*projectResource)(nil)
	_ resource.ResourceWithImportState = (*projectResource)(nil)
)

func New() resource.Resource { return &projectResource{} }

type projectResource struct {
	client *client.Client
}

type model struct {
	ID                    types.String `tfsdk:"id"`
	Name                  types.String `tfsdk:"name"`
	APIKey                types.String `tfsdk:"api_key"`
	APIKeyEnabled         types.Bool   `tfsdk:"api_key_enabled"`
	ReadOnlyAPIKey        types.String `tfsdk:"read_only_api_key"`
	ReadOnlyAPIKeyEnabled types.Bool   `tfsdk:"read_only_api_key_enabled"`
	PingKey               types.String `tfsdk:"ping_key"`
	PingKeyEnabled        types.Bool   `tfsdk:"ping_key_enabled"`
}

func (r *projectResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (r *projectResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Healthchecks project. Project operations use authenticated web endpoints. New projects enable the read-write API key by default.",
		Attributes: map[string]schema.Attribute{
			"id":   schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name": schema.StringAttribute{Required: true},
			"api_key": schema.StringAttribute{
				Computed:  true,
				Sensitive: true,
			},
			"api_key_enabled": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"read_only_api_key": schema.StringAttribute{
				Computed:  true,
				Sensitive: true,
			},
			"read_only_api_key_enabled": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"ping_key": schema.StringAttribute{
				Computed:  true,
				Sensitive: true,
			},
			"ping_key_enabled": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *projectResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*providerdata.ConfiguredClient).Client
}

func (r *projectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	project, err := r.client.CreateProject(ctx, plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to Create Project", err.Error())
		return
	}

	if plan.APIKeyEnabled.IsUnknown() || plan.APIKeyEnabled.IsNull() {
		plan.APIKeyEnabled = types.BoolValue(true)
	}
	if plan.ReadOnlyAPIKeyEnabled.IsUnknown() || plan.ReadOnlyAPIKeyEnabled.IsNull() {
		plan.ReadOnlyAPIKeyEnabled = types.BoolValue(false)
	}
	if plan.PingKeyEnabled.IsUnknown() || plan.PingKeyEnabled.IsNull() {
		plan.PingKeyEnabled = types.BoolValue(false)
	}
	project, err = r.reconcileProjectKeys(ctx, plan, project)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Configure Project Keys", err.Error())
		return
	}

	state := newProjectModel(project)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *projectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	project, err := r.client.GetProject(
		ctx,
		state.ID.ValueString(),
		state.APIKey.ValueString(),
		state.ReadOnlyAPIKey.ValueString(),
		state.PingKey.ValueString(),
	)
	if err != nil {
		if errors.Is(err, client.ErrNotFound()) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to Read Project", err.Error())
		return
	}
	state = newProjectModel(project)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *projectResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan model
	var state model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	project, err := r.client.UpdateProject(ctx, client.Project{
		ID:             state.ID.ValueString(),
		Name:           plan.Name.ValueString(),
		APIKey:         state.APIKey.ValueString(),
		ReadOnlyAPIKey: state.ReadOnlyAPIKey.ValueString(),
		PingKey:        state.PingKey.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to Update Project", err.Error())
		return
	}
	project, err = r.reconcileProjectKeys(ctx, plan, project)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Configure Project Keys", err.Error())
		return
	}
	state = newProjectModel(project)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *projectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteProject(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to Delete Project", err.Error())
	}
}

func (r *projectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *projectResource) reconcileProjectKeys(ctx context.Context, desired model, current *client.Project) (*client.Project, error) {
	changed := false
	for _, key := range []struct {
		name    string
		enabled bool
		want    types.Bool
		value   *string
	}{
		{"api_key", current.APIKeyEnabled, desired.APIKeyEnabled, &current.APIKey},
		{"read_only_api_key", current.ReadOnlyAPIKeyEnabled, desired.ReadOnlyAPIKeyEnabled, &current.ReadOnlyAPIKey},
		{"ping_key", current.PingKeyEnabled, desired.PingKeyEnabled, &current.PingKey},
	} {
		if key.want.IsNull() || key.want.IsUnknown() || key.enabled == key.want.ValueBool() {
			continue
		}
		value, err := r.client.SetProjectKeyEnabled(ctx, current.ID, key.name, key.want.ValueBool())
		if err != nil {
			return nil, err
		}
		*key.value = value
		changed = true
	}
	if !changed {
		return current, nil
	}
	return r.client.GetProject(ctx, current.ID, current.APIKey, current.ReadOnlyAPIKey, current.PingKey)
}

func newProjectModel(project *client.Project) model {
	keyValue := func(enabled bool, value string) types.String {
		if !enabled || value == "" {
			return types.StringNull()
		}
		return types.StringValue(value)
	}
	return model{
		ID:                    types.StringValue(project.ID),
		Name:                  types.StringValue(project.Name),
		APIKey:                keyValue(project.APIKeyEnabled, project.APIKey),
		APIKeyEnabled:         types.BoolValue(project.APIKeyEnabled),
		ReadOnlyAPIKey:        keyValue(project.ReadOnlyAPIKeyEnabled, project.ReadOnlyAPIKey),
		ReadOnlyAPIKeyEnabled: types.BoolValue(project.ReadOnlyAPIKeyEnabled),
		PingKey:               keyValue(project.PingKeyEnabled, project.PingKey),
		PingKeyEnabled:        types.BoolValue(project.PingKeyEnabled),
	}
}
