package integration

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

type ntfyModel struct {
	URL        types.String `tfsdk:"url"`
	Topic      types.String `tfsdk:"topic"`
	Priority   types.Int64  `tfsdk:"priority"`
	PriorityUp types.Int64  `tfsdk:"priority_up"`
	Token      types.String `tfsdk:"token"`
}

func ntfyAttribute() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional:            true,
		MarkdownDescription: "Settings for `type = \"ntfy\"`.",
		Attributes: map[string]schema.Attribute{
			"url":   schema.StringAttribute{Required: true, MarkdownDescription: "Absolute HTTP(S) URL of the ntfy server."},
			"topic": schema.StringAttribute{Required: true, MarkdownDescription: "ntfy topic name."},
			"priority": schema.Int64Attribute{
				Optional: true, Computed: true, Default: int64default.StaticInt64(3),
				MarkdownDescription: "Priority for down notifications, 0–5. Defaults to 3.",
			},
			"priority_up": schema.Int64Attribute{
				Optional: true, Computed: true, Default: int64default.StaticInt64(3),
				MarkdownDescription: "Priority for up notifications, 0–5. Defaults to 3.",
			},
			"token": schema.StringAttribute{Optional: true, Sensitive: true, MarkdownDescription: "Optional ntfy server access token."},
		},
	}
}

func ntfyConfig(ctx context.Context, plan model) (map[string]string, diag.Diagnostics) {
	var diags diag.Diagnostics
	if plan.Ntfy.IsNull() || plan.Ntfy.IsUnknown() {
		diags.AddError("Missing ntfy Configuration", "Set the ntfy block when type is ntfy.")
		return nil, diags
	}
	if (!plan.Config.IsNull() && !plan.Config.IsUnknown()) || (!plan.Webhook.IsNull() && !plan.Webhook.IsUnknown()) {
		diags.AddError("Conflicting ntfy Configuration", "Use the ntfy block for ntfy integrations.")
		return nil, diags
	}
	var settings ntfyModel
	diags.Append(plan.Ntfy.As(ctx, &settings, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return nil, diags
	}
	server, err := url.ParseRequestURI(settings.URL.ValueString())
	if err != nil || server.Host == "" || (server.Scheme != "http" && server.Scheme != "https") || server.User != nil {
		diags.AddError("Invalid ntfy URL", "Set an absolute HTTP(S) URL without credentials. Use ntfy.token for authentication.")
	}
	topic := settings.Topic.ValueString()
	if topic == "" || topic != strings.TrimSpace(topic) {
		diags.AddError("Invalid ntfy Topic", "Set a non-empty topic without leading or trailing whitespace.")
	}
	priority, priorityUp := settings.Priority.ValueInt64(), settings.PriorityUp.ValueInt64()
	if priority < 0 || priority > 5 || priorityUp < 0 || priorityUp > 5 {
		diags.AddError("Invalid ntfy Priority", "Set both priorities between 0 and 5.")
	}
	return map[string]string{
		"url": settings.URL.ValueString(), "topic": topic,
		"priority": strconv.FormatInt(priority, 10), "priority_up": strconv.FormatInt(priorityUp, 10),
		"token": settings.Token.ValueString(),
	}, diags
}

func applyNtfy(ctx context.Context, state *model, config map[string]string) diag.Diagnostics {
	var diags diag.Diagnostics
	var prior ntfyModel
	if !state.Ntfy.IsNull() && !state.Ntfy.IsUnknown() {
		diags.Append(state.Ntfy.As(ctx, &prior, basetypes.ObjectAsOptions{})...)
		if diags.HasError() {
			return diags
		}
	}
	priority, errDown := strconv.ParseInt(config["priority"], 10, 64)
	priorityUp, errUp := strconv.ParseInt(config["priority_up"], 10, 64)
	if errDown != nil || errUp != nil {
		diags.AddError("Invalid ntfy Form", "Healthchecks did not return valid notification priorities.")
		return diags
	}
	token := types.StringValue(config["token"])
	if config["token"] == "" && prior.Token.IsNull() {
		token = types.StringNull()
	}
	settings, objDiags := types.ObjectValueFrom(ctx, state.Ntfy.AttributeTypes(ctx), ntfyModel{
		URL: types.StringValue(config["url"]), Topic: types.StringValue(config["topic"]),
		Priority: types.Int64Value(priority), PriorityUp: types.Int64Value(priorityUp), Token: token,
	})
	diags.Append(objDiags...)
	state.Ntfy = settings
	state.Config = types.MapNull(types.StringType)
	return diags
}
