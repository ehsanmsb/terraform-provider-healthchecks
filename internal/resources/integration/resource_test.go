package integration

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"

	"github.com/ehsanmsb/terraform-provider-healthchecks/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestConfigFromModelWebhookBlock(t *testing.T) {
	ctx := context.Background()

	cfg, diags := configFromModel(ctx, model{
		Type:   types.StringValue("webhook"),
		Config: types.MapNull(types.StringType),
		Webhook: testWebhookObject(t, webhookModel{
			MethodDown: types.StringValue("POST"),
			URLDown:    types.StringValue("https://example.com/down"),
			BodyDown:   types.StringValue(`{"state":"down"}`),
			HeadersDown: types.MapValueMust(types.StringType, map[string]attr.Value{
				"X-Env":           types.StringValue("production"),
				"X-Sample-Header": types.StringValue("$NAME has gone down"),
			}),
		}),
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
		Webhook: testWebhookObject(t, webhookModel{
			URLDown: types.StringValue("https://example.com/down"),
		}),
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

func testWebhookObject(t *testing.T, value webhookModel) types.Object {
	t.Helper()
	ctx := context.Background()
	if value.HeadersDown.IsNull() {
		value.HeadersDown = types.MapNull(types.StringType)
	}
	if value.HeadersUp.IsNull() {
		value.HeadersUp = types.MapNull(types.StringType)
	}
	var resp resource.SchemaResponse
	New().Schema(ctx, resource.SchemaRequest{}, &resp)
	obj, diags := types.ObjectValueFrom(ctx, resp.Schema.Attributes["webhook"].GetType().(types.ObjectType).AttrTypes, value)
	if diags.HasError() {
		t.Fatal(diags)
	}
	return obj
}

func TestEmailStateAcceptsUnknownWebhook(t *testing.T) {
	ctx := context.Background()
	webhook := testWebhookObject(t, webhookModel{})
	state := model{
		Type: types.StringValue("email"),
		Config: mapStringValues(map[string]string{
			"value": "operator@example.com",
			"down":  "true",
			"up":    "true",
		}),
		Webhook: types.ObjectUnknown(webhook.AttributeTypes(ctx)),
	}
	cfg, diags := configFromModel(ctx, state)
	if diags.HasError() {
		t.Fatal(diags)
	}

	for _, tc := range []struct{ input, want types.Object }{
		{state.Webhook, types.ObjectNull(webhook.AttributeTypes(ctx))},
		{webhook, webhook},
	} {
		state.Webhook = tc.input
		diags = applyIntegration(ctx, &state, &client.Integration{Type: "email", Config: cfg})
		if diags.HasError() {
			t.Fatal(diags)
		}
		if !state.Webhook.Equal(tc.want) {
			t.Fatalf("webhook state = %s, want %s", state.Webhook, tc.want)
		}
	}

	state.Name = types.StringValue("Operator")
	if _, diags = configFromModel(ctx, state); !diags.HasError() {
		t.Fatal("expected email name to be rejected before creating the integration")
	}
}

func TestEmailStatePreservesOmittedAndEquivalentFlags(t *testing.T) {
	state := model{
		Config: mapStringValues(map[string]string{
			"value": "operator@example.com",
			"up":    "yes",
		}),
	}
	remote := &client.Integration{
		Type:   "email",
		Config: map[string]string{"value": "operator@example.com", "up": "true", "down": "true"},
	}
	want := state.Config
	if diags := applyIntegration(context.Background(), &state, remote); diags.HasError() {
		t.Fatal(diags)
	}
	if !state.Config.Equal(want) {
		t.Fatalf("email config = %s, want %s", state.Config, want)
	}

	remote.Config["down"] = "false"
	if diags := applyIntegration(context.Background(), &state, remote); diags.HasError() {
		t.Fatal(diags)
	}
	if got := state.Config.Elements()["down"]; got == nil || !got.Equal(types.StringValue("false")) {
		t.Fatal("non-default notification flag was not reported in state")
	}
}
