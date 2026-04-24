package check

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ehsanmsb/terraform-provider-healthchecks/internal/client"
	"github.com/ehsanmsb/terraform-provider-healthchecks/internal/providerdata"
)

var (
	_ resource.Resource                = (*checkResource)(nil)
	_ resource.ResourceWithImportState = (*checkResource)(nil)
)

func New() resource.Resource { return &checkResource{} }

type checkResource struct{ client *client.Client }

type model struct {
	ID        types.String `tfsdk:"id"`
	ProjectID types.String `tfsdk:"project_id"`
	Name      types.String `tfsdk:"name"`
	Slug      types.String `tfsdk:"slug"`
	Tags      types.List   `tfsdk:"tags"`
	Desc      types.String `tfsdk:"desc"`
	Timeout   types.Int64  `tfsdk:"timeout"`
	Grace     types.Int64  `tfsdk:"grace"`
	Schedule  types.String `tfsdk:"schedule"`
	TZ        types.String `tfsdk:"tz"`
	Channels  types.List   `tfsdk:"channels"`
	UUID      types.String `tfsdk:"uuid"`
	PingURL   types.String `tfsdk:"ping_url"`
	Status    types.String `tfsdk:"status"`
}

func (r *checkResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_check"
}

func (r *checkResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Healthchecks check using the Management API v3. The provider obtains a project read-write API key via project settings when necessary.",
		Attributes: map[string]schema.Attribute{
			"id":         schema.StringAttribute{Computed: true},
			"project_id": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"name":       schema.StringAttribute{Required: true},
			"slug":       schema.StringAttribute{Optional: true},
			"tags":       schema.ListAttribute{Optional: true, ElementType: types.StringType, Computed: true, Default: listdefault.StaticValue(types.ListValueMust(types.StringType, nil))},
			"desc":       schema.StringAttribute{Optional: true},
			"timeout":    schema.Int64Attribute{Optional: true},
			"grace":      schema.Int64Attribute{Optional: true},
			"schedule":   schema.StringAttribute{Optional: true},
			"tz":         schema.StringAttribute{Optional: true},
			"channels":   schema.ListAttribute{Optional: true, ElementType: types.StringType, Computed: true, Default: listdefault.StaticValue(types.ListValueMust(types.StringType, nil))},
			"uuid":       schema.StringAttribute{Computed: true},
			"ping_url":   schema.StringAttribute{Computed: true},
			"status":     schema.StringAttribute{Computed: true},
		},
	}
}

func (r *checkResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*providerdata.ConfiguredClient).Client
}

func (r *checkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	check, err := r.client.CreateCheck(ctx, plan.ProjectID.ValueString(), buildPayload(plan))
	if err != nil {
		resp.Diagnostics.AddError("Unable to Create Check", err.Error())
		return
	}
	state := plan
	applyCheckToState(&state, check)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *checkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	check, err := r.client.GetCheck(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil {
		if errors.Is(err, client.ErrNotFound()) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to Read Check", err.Error())
		return
	}
	applyCheckToState(&state, check)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *checkResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan model
	var state model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	check, err := r.client.UpdateCheck(ctx, state.ProjectID.ValueString(), state.ID.ValueString(), buildPayload(plan))
	if err != nil {
		resp.Diagnostics.AddError("Unable to Update Check", err.Error())
		return
	}
	state = plan
	state.ID = types.StringValue(check.ID)
	applyCheckToState(&state, check)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *checkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteCheck(ctx, state.ProjectID.ValueString(), state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to Delete Check", err.Error())
	}
}

func (r *checkResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid Import ID", "Use `project_id/check_uuid`.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

func buildPayload(m model) map[string]any {
	payload := map[string]any{"name": m.Name.ValueString()}
	if !m.Slug.IsNull() && m.Slug.ValueString() != "" {
		payload["slug"] = m.Slug.ValueString()
	}
	if !m.Desc.IsNull() {
		payload["desc"] = m.Desc.ValueString()
	}
	if !m.Timeout.IsNull() {
		payload["timeout"] = m.Timeout.ValueInt64()
	}
	if !m.Grace.IsNull() {
		payload["grace"] = m.Grace.ValueInt64()
	}
	if !m.Schedule.IsNull() && m.Schedule.ValueString() != "" {
		payload["schedule"] = m.Schedule.ValueString()
	}
	if !m.TZ.IsNull() && m.TZ.ValueString() != "" {
		payload["tz"] = m.TZ.ValueString()
	}
	if !m.Tags.IsNull() && m.Tags.Elements() != nil {
		var tags []string
		_ = m.Tags.ElementsAs(context.Background(), &tags, false)
		payload["tags"] = strings.Join(tags, " ")
	}
	if !m.Channels.IsNull() && m.Channels.Elements() != nil {
		var channels []string
		_ = m.Channels.ElementsAs(context.Background(), &channels, false)
		payload["channels"] = strings.Join(channels, ",")
	}
	return payload
}

func applyCheckToState(state *model, check *client.Check) {
	state.ID = types.StringValue(check.ID)
	state.UUID = types.StringValue(check.ID)
	state.Name = types.StringValue(check.Name)
	state.Slug = types.StringValue(check.Slug)
	if !(state.Desc.IsNull() && check.Desc == "") {
		state.Desc = types.StringValue(check.Desc)
	}
	state.Grace = types.Int64Value(check.Grace)
	state.Status = types.StringValue(check.Status)
	state.PingURL = types.StringValue(check.PingURL)
	if check.Timeout != nil {
		state.Timeout = types.Int64Value(*check.Timeout)
	}
	if check.Schedule != nil {
		state.Schedule = types.StringValue(*check.Schedule)
	}
	if check.TZ != nil {
		state.TZ = types.StringValue(*check.TZ)
	}
	state.Tags = splitSpaceList(check.Tags)
	state.Channels = normalizeChannels(state.Channels, check.Channels)
}

func splitSpaceList(v string) types.List {
	if strings.TrimSpace(v) == "" {
		return types.ListValueMust(types.StringType, nil)
	}
	parts := strings.Fields(v)
	values := make([]attr.Value, 0, len(parts))
	for _, part := range parts {
		values = append(values, types.StringValue(part))
	}
	return types.ListValueMust(types.StringType, values)
}

func splitCommaList(v string) types.List {
	if strings.TrimSpace(v) == "" {
		return types.ListValueMust(types.StringType, nil)
	}
	parts := strings.Split(v, ",")
	values := make([]attr.Value, 0, len(parts))
	for _, part := range parts {
		values = append(values, types.StringValue(strings.TrimSpace(part)))
	}
	return types.ListValueMust(types.StringType, values)
}

func normalizeChannels(current types.List, raw string) types.List {
	if strings.TrimSpace(raw) == "" {
		return types.ListValueMust(types.StringType, nil)
	}

	server := splitCommaStrings(raw)
	if !current.IsNull() && current.Elements() != nil {
		var existing []string
		_ = current.ElementsAs(context.Background(), &existing, false)
		if sameStringSet(existing, server) {
			return current
		}
	}

	slices.Sort(server)
	values := make([]attr.Value, 0, len(server))
	for _, part := range server {
		values = append(values, types.StringValue(part))
	}
	return types.ListValueMust(types.StringType, values)
}

func splitCommaStrings(v string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	counts := map[string]int{}
	for _, item := range a {
		counts[item]++
	}
	for _, item := range b {
		counts[item]--
		if counts[item] < 0 {
			return false
		}
	}
	for _, count := range counts {
		if count != 0 {
			return false
		}
	}
	return true
}
