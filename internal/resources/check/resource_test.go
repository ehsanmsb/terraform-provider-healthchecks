package check

import (
	"testing"

	"github.com/ehsanmsb/terraform-provider-healthchecks/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestCheckDefaultsComeFromHealthchecks(t *testing.T) {
	state := model{
		Slug:     types.StringUnknown(),
		Grace:    types.Int64Unknown(),
		Timeout:  types.Int64Unknown(),
		TZ:       types.StringUnknown(),
		Tags:     types.ListUnknown(types.StringType),
		Channels: types.ListUnknown(types.StringType),
	}
	payload := buildPayload(state)
	for _, field := range []string{"slug", "grace", "timeout", "tz", "tags", "channels"} {
		if _, exists := payload[field]; exists {
			t.Fatalf("payload contains unknown %s", field)
		}
	}

	applyCheckToState(&state, &client.Check{Grace: 3600})
	if state.Grace.ValueInt64() != 3600 || state.Slug.IsUnknown() || !state.Timeout.IsNull() || !state.TZ.IsNull() {
		t.Fatalf("unexpected check defaults: slug %s, grace %s, timeout %s, timezone %s", state.Slug, state.Grace, state.Timeout, state.TZ)
	}
}

func TestExplicitEmptyCheckListsClearValues(t *testing.T) {
	payload := buildPayload(model{
		Name:     types.StringValue("test"),
		Tags:     types.ListValueMust(types.StringType, nil),
		Channels: types.ListValueMust(types.StringType, nil),
	})
	for _, field := range []string{"tags", "channels"} {
		if value, ok := payload[field]; !ok || value != "" {
			t.Fatalf("explicitly clearing %s produced %v", field, value)
		}
	}
}
