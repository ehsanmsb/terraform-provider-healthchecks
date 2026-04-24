package project

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
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
		MarkdownDescription: "Manages a Healthchecks project. Project operations use authenticated web endpoints, and the provider ensures a read-write project API key exists for downstream API usage.",
		Attributes: map[string]schema.Attribute{
			"id":   schema.StringAttribute{Computed: true},
			"name": schema.StringAttribute{Required: true},
			"api_key": schema.StringAttribute{
				Computed:  true,
				Sensitive: true,
			},
			"api_key_enabled": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(true),
			},
			"read_only_api_key": schema.StringAttribute{
				Computed:  true,
				Sensitive: true,
			},
			"read_only_api_key_enabled": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"ping_key": schema.StringAttribute{
				Computed:  true,
				Sensitive: true,
			},
			"ping_key_enabled": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
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

	project, err = r.reconcileProjectKeys(ctx, project.ID, plan, project)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Configure Project Keys", err.Error())
		return
	}

	state := newProjectModel(plan, project)
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
	state = newProjectModel(state, project)
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
	project, err := r.client.UpdateProject(ctx, state.ID.ValueString(), plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to Update Project", err.Error())
		return
	}
	project, err = r.reconcileProjectKeys(ctx, state.ID.ValueString(), plan, project)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Configure Project Keys", err.Error())
		return
	}
	state = newProjectModel(plan, project)
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

func (r *projectResource) reconcileProjectKeys(ctx context.Context, projectID string, desired model, current *client.Project) (*client.Project, error) {
	if current == nil {
		current = &client.Project{ID: projectID}
	}

	apiKey, err := r.client.SetProjectKeyEnabled(ctx, projectID, "api_key", desired.APIKeyEnabled.ValueBool())
	if err != nil {
		return nil, err
	}
	current.APIKeyEnabled = desired.APIKeyEnabled.ValueBool()
	current.APIKey = apiKey

	readOnlyKey, err := r.client.SetProjectKeyEnabled(ctx, projectID, "read_only_api_key", desired.ReadOnlyAPIKeyEnabled.ValueBool())
	if err != nil {
		return nil, err
	}
	current.ReadOnlyAPIKeyEnabled = desired.ReadOnlyAPIKeyEnabled.ValueBool()
	current.ReadOnlyAPIKey = readOnlyKey

	pingKey, err := r.client.SetProjectKeyEnabled(ctx, projectID, "ping_key", desired.PingKeyEnabled.ValueBool())
	if err != nil {
		return nil, err
	}
	current.PingKeyEnabled = desired.PingKeyEnabled.ValueBool()
	current.PingKey = pingKey

	return r.client.GetProject(ctx, projectID, current.APIKey, current.ReadOnlyAPIKey, current.PingKey)
}

func newProjectModel(previous model, project *client.Project) model {
	state := previous
	state.ID = types.StringValue(project.ID)
	state.Name = types.StringValue(project.Name)
	state.APIKeyEnabled = types.BoolValue(project.APIKeyEnabled)
	state.ReadOnlyAPIKeyEnabled = types.BoolValue(project.ReadOnlyAPIKeyEnabled)
	state.PingKeyEnabled = types.BoolValue(project.PingKeyEnabled)

	if project.APIKeyEnabled {
		if project.APIKey != "" {
			state.APIKey = types.StringValue(project.APIKey)
		}
	} else {
		state.APIKey = types.StringNull()
	}

	if project.ReadOnlyAPIKeyEnabled {
		if project.ReadOnlyAPIKey != "" {
			state.ReadOnlyAPIKey = types.StringValue(project.ReadOnlyAPIKey)
		}
	} else {
		state.ReadOnlyAPIKey = types.StringNull()
	}

	if project.PingKeyEnabled {
		if project.PingKey != "" {
			state.PingKey = types.StringValue(project.PingKey)
		}
	} else {
		state.PingKey = types.StringNull()
	}

	return state
}
