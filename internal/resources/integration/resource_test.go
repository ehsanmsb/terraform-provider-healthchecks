package integration

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestConfigFromModelWebhookBlock(t *testing.T) {
	ctx := context.Background()

	cfg, diags := configFromModel(ctx, model{
		Type:   types.StringValue("webhook"),
		Config: types.MapNull(types.StringType),
		Webhook: webhookModel{
			MethodDown: types.StringValue("POST"),
			URLDown:    types.StringValue("https://example.com/down"),
			BodyDown:   types.StringValue(`{"state":"down"}`),
			HeadersDown: types.MapValueMust(types.StringType, map[string]attr.Value{
				"X-Env":           types.StringValue("production"),
				"X-Sample-Header": types.StringValue("$NAME has gone down"),
			}),
		},
	})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %#v", diags)
	}

	if cfg["method_down"] != "POST" {
		t.Fatalf("unexpected method_down: %q", cfg["method_down"])
	}
	if cfg["url_down"] != "https://example.com/down" {
		t.Fatalf("unexpected url_down: %q", cfg["url_down"])
	}
	if cfg["headers_down"] != "X-Env: production\nX-Sample-Header: $NAME has gone down" {
		t.Fatalf("unexpected headers_down: %q", cfg["headers_down"])
	}
}

func TestConfigFromModelRejectsConflictingWebhookInputs(t *testing.T) {
	ctx := context.Background()

	_, diags := configFromModel(ctx, model{
		Type: types.StringValue("webhook"),
		Config: types.MapValueMust(types.StringType, map[string]attr.Value{
			"url_down": types.StringValue("https://example.com/down"),
		}),
		Webhook: webhookModel{
			URLDown: types.StringValue("https://example.com/down"),
		},
	})
	if !diags.HasError() {
		t.Fatal("expected diagnostics for conflicting webhook inputs")
	}
}

func TestParseHeaderLines(t *testing.T) {
	headers := parseHeaderLines("X-Env: production\nX-Sample-Header: $NAME has gone down")

	if headers["X-Env"] != "production" {
		t.Fatalf("unexpected X-Env header: %q", headers["X-Env"])
	}
	if headers["X-Sample-Header"] != "$NAME has gone down" {
		t.Fatalf("unexpected X-Sample-Header: %q", headers["X-Sample-Header"])
	}
}

func TestMapToStringsAllowsUnknownMap(t *testing.T) {
	ctx := context.Background()

	values, err := mapToStrings(ctx, types.MapUnknown(types.StringType))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(values) != 0 {
		t.Fatalf("expected empty map, got %#v", values)
	}
}
