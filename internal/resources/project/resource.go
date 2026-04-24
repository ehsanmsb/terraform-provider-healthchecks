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
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	APIKey        types.String `tfsdk:"api_key"`
	APIKeyEnabled types.Bool   `tfsdk:"api_key_enabled"`
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
	state := model{
		ID:            types.StringValue(project.ID),
		Name:          types.StringValue(project.Name),
		APIKey:        types.StringValue(project.APIKey),
		APIKeyEnabled: types.BoolValue(project.APIKeyEnabled),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *projectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	project, err := r.client.GetProject(ctx, state.ID.ValueString(), state.APIKey.ValueString())
	if err != nil {
		if errors.Is(err, client.ErrNotFound()) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to Read Project", err.Error())
		return
	}
	if state.APIKey.IsNull() || state.APIKey.ValueString() == "" {
		if key, err := r.client.EnsureProjectAPIKey(ctx, state.ID.ValueString()); err == nil {
			project.APIKey = key
			project.APIKeyEnabled = true
		}
	}
	state.Name = types.StringValue(project.Name)
	state.APIKeyEnabled = types.BoolValue(project.APIKeyEnabled)
	if project.APIKey != "" {
		state.APIKey = types.StringValue(project.APIKey)
	}
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
	state.Name = types.StringValue(project.Name)
	state.APIKeyEnabled = plan.APIKeyEnabled
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
