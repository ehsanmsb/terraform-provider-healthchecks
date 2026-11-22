package integration

import (
	"context"
	"errors"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	"github.com/ehsanmsb/terraform-provider-healthchecks/internal/client"
	"github.com/ehsanmsb/terraform-provider-healthchecks/internal/providerdata"
)

var (
	_ resource.Resource                   = (*integrationResource)(nil)
	_ resource.ResourceWithImportState    = (*integrationResource)(nil)
	_ resource.ResourceWithValidateConfig = (*integrationResource)(nil)
)

func New() resource.Resource { return &integrationResource{} }

type integrationResource struct{ client *client.Client }

type model struct {
	ID            types.String `tfsdk:"id"`
	ProjectID     types.String `tfsdk:"project_id"`
	ProjectAPIKey types.String `tfsdk:"project_api_key"`
	Type          types.String `tfsdk:"type"`
	Name          types.String `tfsdk:"name"`
	Config        types.Map    `tfsdk:"config"`
	Webhook       types.Object `tfsdk:"webhook"`
	Ntfy          types.Object `tfsdk:"ntfy"`
}

func (r *integrationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration"
}

func (r *integrationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages project integrations. The current implementation supports `webhook`, `email`, and `ntfy` channels through authenticated web form endpoints.",
		Attributes: map[string]schema.Attribute{
			"id":         schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"project_id": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"project_api_key": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Read-write API key used to identify the channel after creating the integration. Reference the project resource's api_key.",
			},
			"type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Integration type. Changing type replaces the channel.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{Optional: true, MarkdownDescription: "Name of a webhook integration. Omit for email and ntfy integrations."},
			"config": schema.MapAttribute{
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Legacy generic integration config map. Still supported for `type = \"email\"` and webhook backward compatibility, but the `webhook` block is the preferred interface for webhook integrations.",
			},
			"ntfy": ntfyAttribute(),
			"webhook": schema.SingleNestedAttribute{
				Optional:            true,
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

func (r *integrationResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var integrationType, name types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("type"), &integrationType)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("name"), &name)...)
	validateIntegrationName(integrationType, name, &resp.Diagnostics)
}

func validateIntegrationName(integrationType, name types.String, diags *diag.Diagnostics) {
	if integrationType.ValueString() == "email" && name.ValueString() != "" {
		diags.AddAttributeError(path.Root("name"), "Name Not Supported for Email Integrations", "Omit name when type is email.")
	}
	if integrationType.ValueString() == "ntfy" && name.ValueString() != "" {
		diags.AddAttributeError(path.Root("name"), "Name Not Supported for ntfy Integrations", "Omit name when type is ntfy.")
	}
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
		ProjectID:     plan.ProjectID.ValueString(),
		ProjectAPIKey: plan.ProjectAPIKey.ValueString(),
		Type:          plan.Type.ValueString(),
		Name:          plan.Name.ValueString(),
		Config:        cfg,
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to Create Integration", err.Error())
		return
	}

	state := plan
	resp.Diagnostics.Append(applyIntegration(ctx, &state, integration)...)
	if resp.Diagnostics.HasError() {
		return
	}
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

	resp.Diagnostics.Append(applyIntegration(ctx, &state, integration)...)
	if resp.Diagnostics.HasError() {
		return
	}
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
	resp.Diagnostics.Append(applyIntegration(ctx, &state, integration)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *integrationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteIntegration(ctx, state.ProjectID.ValueString(), state.ID.ValueString()); err != nil {
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

func configFromModel(ctx context.Context, plan model) (map[string]string, diag.Diagnostics) {
	var diags diag.Diagnostics
	validateIntegrationName(plan.Type, plan.Name, &diags)
	if diags.HasError() {
		return nil, diags
	}

	if plan.Type.ValueString() == "ntfy" {
		return ntfyConfig(ctx, plan)
	}
	if !plan.Ntfy.IsNull() && !plan.Ntfy.IsUnknown() {
		diags.AddError("ntfy Block Not Allowed", "Use the ntfy block with type ntfy.")
		return nil, diags
	}

	var webhook webhookModel
	if !plan.Webhook.IsNull() && !plan.Webhook.IsUnknown() {
		diags.Append(plan.Webhook.As(ctx, &webhook, basetypes.ObjectAsOptions{})...)
		if diags.HasError() {
			return nil, diags
		}
	}

	legacyConfig, err := mapToStrings(ctx, plan.Config)
	if err != nil {
		diags.AddError("Invalid Integration Config", err.Error())
		return nil, diags
	}

	switch plan.Type.ValueString() {
	case "webhook":
		hasWebhook := webhookHasValues(webhook)
		hasLegacy := !plan.Config.IsNull() && !plan.Config.IsUnknown()

		if hasWebhook && hasLegacy {
			diags.AddError(
				"Conflicting Webhook Configuration",
				"Use either the structured `webhook` block or the legacy `config` map for `type = \"webhook\"`, but not both.",
			)
			return nil, diags
		}
		if hasWebhook {
			webhookConfig, err := webhookToConfig(ctx, webhook)
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
		if webhookHasValues(webhook) {
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

func applyIntegration(ctx context.Context, state *model, in *client.Integration) diag.Diagnostics {
	state.ID = types.StringValue(in.ID)
	state.ProjectID = types.StringValue(in.ProjectID)
	state.Type = types.StringValue(in.Type)
	if !(state.Name.IsNull() && in.Name == "") {
		state.Name = types.StringValue(in.Name)
	}

	if in.Type == "ntfy" {
		return applyNtfy(ctx, state, in.Config)
	}

	values := map[string]attr.Value{}
	configured := state.Config.Elements()
	configKnown := !state.Config.IsNull() && !state.Config.IsUnknown()
	for key, value := range in.Config {
		if in.Type == "email" && (key == "up" || key == "down") && configKnown {
			prior, exists := configured[key].(types.String)
			if client.ParseBoolString(prior.ValueString(), true) == (value == "true") {
				if exists {
					values[key] = prior
				}
				continue
			}
		}
		values[key] = types.StringValue(value)
	}
	if len(values) == 0 {
		state.Config = types.MapNull(types.StringType)
	} else {
		state.Config = types.MapValueMust(types.StringType, values)
	}

	return applyWebhook(ctx, state, in)
}

func (r *integrationResource) createIntegration(ctx context.Context, in client.Integration) (*client.Integration, error) {
	switch in.Type {
	case "webhook":
		return r.client.CreateWebhookIntegration(ctx, in)
	case "email":
		return r.client.CreateEmailIntegration(ctx, in)
	case "ntfy":
		return r.client.CreateNtfyIntegration(ctx, in)
	default:
		return nil, errors.New("unsupported integration type: use `webhook`, `email`, or `ntfy`")
	}
}

func (r *integrationResource) getIntegration(ctx context.Context, projectID, channelID, integrationType string) (*client.Integration, error) {
	switch integrationType {
	case "webhook":
		return r.client.GetWebhookIntegration(ctx, projectID, channelID)
	case "email":
		return r.client.GetEmailIntegration(ctx, projectID, channelID)
	case "ntfy":
		return r.client.GetNtfyIntegration(ctx, projectID, channelID)
	default:
		return nil, errors.New("unsupported integration type: use `webhook`, `email`, or `ntfy`")
	}
}

func (r *integrationResource) updateIntegration(ctx context.Context, in client.Integration) (*client.Integration, error) {
	switch in.Type {
	case "webhook":
		return r.client.UpdateWebhookIntegration(ctx, in)
	case "email":
		return r.client.UpdateEmailIntegration(ctx, in)
	case "ntfy":
		return r.client.UpdateNtfyIntegration(ctx, in)
	default:
		return nil, errors.New("unsupported integration type: use `webhook`, `email`, or `ntfy`")
	}
}
