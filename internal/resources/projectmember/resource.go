package projectmember

import (
	"context"
	"errors"
	"strings"

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
	_ resource.Resource                = (*resourceImpl)(nil)
	_ resource.ResourceWithImportState = (*resourceImpl)(nil)
)

func New() resource.Resource { return &resourceImpl{} }

type resourceImpl struct{ client *client.Client }

type model struct {
	ID        types.String `tfsdk:"id"`
	ProjectID types.String `tfsdk:"project_id"`
	Email     types.String `tfsdk:"email"`
	Role      types.String `tfsdk:"role"`
}

func (r *resourceImpl) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_member"
}

func (r *resourceImpl) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages project team membership through the project settings web form. Role updates are implemented as remove-and-reinvite because Healthchecks currently exposes invite/remove actions, not a dedicated role update endpoint.",
		Attributes: map[string]schema.Attribute{
			"id":         schema.StringAttribute{Computed: true},
			"project_id": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"email":      schema.StringAttribute{Required: true},
			"role":       schema.StringAttribute{Required: true},
		},
	}
}

func (r *resourceImpl) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*providerdata.ConfiguredClient).Client
}

func (r *resourceImpl) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	member, err := r.client.UpsertProjectMember(ctx, plan.ProjectID.ValueString(), strings.ToLower(plan.Email.ValueString()), plan.Role.ValueString(), nil)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Create Project Member", err.Error())
		return
	}
	state := model{
		ID:        types.StringValue(member.ProjectID + "/" + member.Email),
		ProjectID: types.StringValue(member.ProjectID),
		Email:     types.StringValue(member.Email),
		Role:      types.StringValue(member.Role),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *resourceImpl) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	member, err := r.client.GetProjectMember(ctx, state.ProjectID.ValueString(), state.Email.ValueString())
	if err != nil {
		if errors.Is(err, client.ErrNotFound()) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to Read Project Member", err.Error())
		return
	}
	state.Email = types.StringValue(member.Email)
	state.Role = types.StringValue(member.Role)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *resourceImpl) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan model
	var state model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	existing := &client.ProjectMember{ProjectID: state.ProjectID.ValueString(), Email: state.Email.ValueString(), Role: state.Role.ValueString()}
	member, err := r.client.UpsertProjectMember(ctx, plan.ProjectID.ValueString(), strings.ToLower(plan.Email.ValueString()), plan.Role.ValueString(), existing)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Update Project Member", err.Error())
		return
	}
	state.ID = types.StringValue(member.ProjectID + "/" + member.Email)
	state.Email = types.StringValue(member.Email)
	state.Role = types.StringValue(member.Role)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *resourceImpl) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.RemoveProjectMember(ctx, state.ProjectID.ValueString(), state.Email.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to Delete Project Member", err.Error())
	}
}

func (r *resourceImpl) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid Import ID", "Use `project_id/email`.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("email"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
