package integration

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ehsanmsb/terraform-provider-healthchecks/internal/client"
)

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

func applyWebhook(ctx context.Context, state *model, in *client.Integration) diag.Diagnostics {
	if in.Type != "webhook" {
		if state.Webhook.IsUnknown() {
			state.Webhook = types.ObjectNull(state.Webhook.AttributeTypes(ctx))
		}
		return nil
	}

	if state.Webhook.IsNull() {
		return nil
	}
	webhook, diags := types.ObjectValueFrom(ctx, state.Webhook.AttributeTypes(ctx), webhookModel{
		MethodDown:  stringOrNull(in.Config["method_down"]),
		URLDown:     stringOrNull(in.Config["url_down"]),
		BodyDown:    stringOrNull(in.Config["body_down"]),
		HeadersDown: mapStringValues(parseHeaderLines(in.Config["headers_down"])),
		MethodUp:    stringOrNull(in.Config["method_up"]),
		URLUp:       stringOrNull(in.Config["url_up"]),
		BodyUp:      stringOrNull(in.Config["body_up"]),
		HeadersUp:   mapStringValues(parseHeaderLines(in.Config["headers_up"])),
	})
	state.Webhook = webhook
	return diags
}

func stringOrNull(value string) types.String {
	if strings.TrimSpace(value) == "" {
		return types.StringNull()
	}
	return types.StringValue(value)
}
