package integration

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
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
	Webhook   webhookModel `tfsdk:"webhook"`
}

type webhookModel struct {
	MethodDown  types.String `tfsdk:"method_down"`
	URLDown     types.String `tfsdk:"url_down"`
	BodyDown    types.String `tfsdk:"body_down"`
	HeadersDown types.Map    `tfsdk:"headers_down"`
	MethodUp    types.String `tfsdk:"method_up"`
	URLUp       types.String `tfsdk:"url_up"`
	BodyUp      types.String `tfsdk:"body_up"`
	HeadersUp   types.Map    `tfsdk:"headers_up"`
}

func (r *integrationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration"
}

func (r *integrationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages project integrations. The current implementation supports `webhook` and `email` channels through authenticated web form endpoints.",
		Attributes: map[string]schema.Attribute{
			"id":         schema.StringAttribute{Computed: true},
			"project_id": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"type":       schema.StringAttribute{Required: true},
			"name":       schema.StringAttribute{Optional: true},
			"config": schema.MapAttribute{
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Legacy generic integration config map. Still supported for `type = \"email\"` and webhook backward compatibility, but the `webhook` block is the preferred interface for webhook integrations.",
			},
			"webhook": schema.SingleNestedAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Structured webhook configuration. Preferred over the legacy `config` map for `type = \"webhook\"`.",
				Attributes: map[string]schema.Attribute{
					"method_down":  schema.StringAttribute{Optional: true},
					"url_down":     schema.StringAttribute{Optional: true},
					"body_down":    schema.StringAttribute{Optional: true},
					"headers_down": schema.MapAttribute{Optional: true, ElementType: types.StringType},
					"method_up":    schema.StringAttribute{Optional: true},
					"url_up":       schema.StringAttribute{Optional: true},
					"body_up":      schema.StringAttribute{Optional: true},
					"headers_up":   schema.MapAttribute{Optional: true, ElementType: types.StringType},
				},
			},
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

	cfg, diags := configFromModel(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	integration, err := r.createIntegration(ctx, client.Integration{
		ProjectID: plan.ProjectID.ValueString(),
		Type:      plan.Type.ValueString(),
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

	integration, err := r.getIntegration(ctx, state.ProjectID.ValueString(), state.ID.ValueString(), state.Type.ValueString())
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

	cfg, diags := configFromModel(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	integration, err := r.updateIntegration(ctx, client.Integration{
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
	if len(parts) != 3 {
		resp.Diagnostics.AddError("Invalid Import ID", "Use `project_id/type/channel_id`.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("type"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[2])...)
}

func mapToStrings(ctx context.Context, m types.Map) (map[string]string, error) {
	out := map[string]string{}
	if m.IsNull() || m.IsUnknown() {
		return out, nil
	}
	diags := m.ElementsAs(ctx, &out, false)
	if diags.HasError() {
		return nil, errors.New(diags.Errors()[0].Summary())
	}
	return out, nil
}

func webhookHasValues(m webhookModel) bool {
	for _, v := range []attr.Value{
		m.MethodDown,
		m.URLDown,
		m.BodyDown,
		m.HeadersDown,
		m.MethodUp,
		m.URLUp,
		m.BodyUp,
		m.HeadersUp,
	} {
		if !v.IsNull() && !v.IsUnknown() {
			return true
		}
	}
	return false
}

func configFromModel(ctx context.Context, plan model) (map[string]string, diag.Diagnostics) {
	var diags diag.Diagnostics

	legacyConfig, err := mapToStrings(ctx, plan.Config)
	if err != nil {
		diags.AddError("Invalid Integration Config", err.Error())
		return nil, diags
	}

	switch plan.Type.ValueString() {
	case "webhook":
		hasWebhook := webhookHasValues(plan.Webhook)
		hasLegacy := !plan.Config.IsNull() && !plan.Config.IsUnknown()

		if hasWebhook && hasLegacy {
			diags.AddError(
				"Conflicting Webhook Configuration",
				"Use either the structured `webhook` block or the legacy `config` map for `type = \"webhook\"`, but not both.",
			)
			return nil, diags
		}
		if hasWebhook {
			webhookConfig, err := webhookToConfig(ctx, plan.Webhook)
			if err != nil {
				diags.AddError("Invalid Webhook Configuration", err.Error())
				return nil, diags
			}
			return webhookConfig, diags
		}
		if hasLegacy {
			return legacyConfig, diags
		}

		diags.AddError(
			"Missing Webhook Configuration",
			"Set either the structured `webhook` block or the legacy `config` map when `type = \"webhook\"`.",
		)
		return nil, diags
	case "email":
		if webhookHasValues(plan.Webhook) {
			diags.AddError(
				"Webhook Block Not Allowed",
				"The `webhook` block can only be used when `type = \"webhook\"`.",
			)
			return nil, diags
		}
		if plan.Config.IsNull() || plan.Config.IsUnknown() {
			diags.AddError(
				"Missing Email Configuration",
				"Set the `config` map when `type = \"email\"`.",
			)
			return nil, diags
		}
		return legacyConfig, diags
	default:
		return legacyConfig, diags
	}
}

func webhookToConfig(ctx context.Context, webhook webhookModel) (map[string]string, error) {
	config := map[string]string{}

	for key, value := range map[string]types.String{
		"method_down": webhook.MethodDown,
		"url_down":    webhook.URLDown,
		"body_down":   webhook.BodyDown,
		"method_up":   webhook.MethodUp,
		"url_up":      webhook.URLUp,
		"body_up":     webhook.BodyUp,
	} {
		if !value.IsNull() && !value.IsUnknown() && strings.TrimSpace(value.ValueString()) != "" {
			config[key] = value.ValueString()
		}
	}

	for key, value := range map[string]types.Map{
		"headers_down": webhook.HeadersDown,
		"headers_up":   webhook.HeadersUp,
	} {
		headers, err := mapToStrings(ctx, value)
		if err != nil {
			return nil, err
		}
		if len(headers) == 0 {
			continue
		}
		config[key] = joinHeaderLines(headers)
	}

	return config, nil
}

func joinHeaderLines(headers map[string]string) string {
	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, fmt.Sprintf("%s: %s", key, headers[key]))
	}

	return strings.Join(lines, "\n")
}

func parseHeaderLines(value string) map[string]string {
	headers := map[string]string{}
	for _, line := range strings.Split(value, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		headers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return headers
}

func mapStringValues(values map[string]string) types.Map {
	if len(values) == 0 {
		return types.MapNull(types.StringType)
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	attrs := make(map[string]attr.Value, len(values))
	for _, key := range keys {
		attrs[key] = types.StringValue(values[key])
	}

	return types.MapValueMust(types.StringType, attrs)
}

func applyIntegration(state *model, in *client.Integration) {
	state.ID = types.StringValue(in.ID)
	state.ProjectID = types.StringValue(in.ProjectID)
	state.Type = types.StringValue(in.Type)
	if !(state.Name.IsNull() && in.Name == "") {
		state.Name = types.StringValue(in.Name)
	}

	keys := make([]string, 0, len(in.Config))
	for key := range in.Config {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	values := map[string]attr.Value{}
	for _, key := range keys {
		values[key] = types.StringValue(in.Config[key])
	}
	if len(values) == 0 {
		state.Config = types.MapNull(types.StringType)
	} else {
		state.Config = types.MapValueMust(types.StringType, values)
	}

	if in.Type != "webhook" {
		state.Webhook = webhookModel{
			MethodDown:  types.StringNull(),
			URLDown:     types.StringNull(),
			BodyDown:    types.StringNull(),
			HeadersDown: types.MapNull(types.StringType),
			MethodUp:    types.StringNull(),
			URLUp:       types.StringNull(),
			BodyUp:      types.StringNull(),
			HeadersUp:   types.MapNull(types.StringType),
		}
		return
	}

	state.Webhook = webhookModel{
		MethodDown:  stringOrNull(in.Config["method_down"]),
		URLDown:     stringOrNull(in.Config["url_down"]),
		BodyDown:    stringOrNull(in.Config["body_down"]),
		HeadersDown: mapStringValues(parseHeaderLines(in.Config["headers_down"])),
		MethodUp:    stringOrNull(in.Config["method_up"]),
		URLUp:       stringOrNull(in.Config["url_up"]),
		BodyUp:      stringOrNull(in.Config["body_up"]),
		HeadersUp:   mapStringValues(parseHeaderLines(in.Config["headers_up"])),
	}
}

func stringOrNull(value string) types.String {
	if strings.TrimSpace(value) == "" {
		return types.StringNull()
	}
	return types.StringValue(value)
}

func (r *integrationResource) createIntegration(ctx context.Context, in client.Integration) (*client.Integration, error) {
	switch in.Type {
	case "webhook":
		return r.client.CreateWebhookIntegration(ctx, in)
	case "email":
		return r.client.CreateEmailIntegration(ctx, in)
	default:
		return nil, errors.New("unsupported integration type: only `webhook` and `email` are currently implemented")
	}
}

func (r *integrationResource) getIntegration(ctx context.Context, projectID, channelID, integrationType string) (*client.Integration, error) {
	switch integrationType {
	case "webhook":
		return r.client.GetWebhookIntegration(ctx, projectID, channelID)
	case "email":
		return r.client.GetEmailIntegration(ctx, projectID, channelID)
	default:
		return nil, errors.New("unsupported integration type: only `webhook` and `email` are currently implemented")
	}
}

func (r *integrationResource) updateIntegration(ctx context.Context, in client.Integration) (*client.Integration, error) {
	switch in.Type {
	case "webhook":
		return r.client.UpdateWebhookIntegration(ctx, in)
	case "email":
		return r.client.UpdateEmailIntegration(ctx, in)
	default:
		return nil, errors.New("unsupported integration type: only `webhook` and `email` are currently implemented")
	}
}
