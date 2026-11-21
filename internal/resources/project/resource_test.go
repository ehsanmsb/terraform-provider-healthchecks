package project

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ehsanmsb/terraform-provider-healthchecks/internal/client"
)

func TestImportedProjectPreservesEnabledKeys(t *testing.T) {
	current := &client.Project{
		ID:                    "project",
		APIKeyEnabled:         true,
		ReadOnlyAPIKeyEnabled: true,
		PingKeyEnabled:        true,
		PingKey:               "known-ping-key",
	}
	r := &projectResource{}
	updated, err := r.reconcileProjectKeys(context.Background(), model{
		APIKeyEnabled:         types.BoolValue(true),
		ReadOnlyAPIKeyEnabled: types.BoolUnknown(),
	}, current)
	if err != nil {
		t.Fatal(err)
	}

	state := newProjectModel(updated)
	if !state.APIKeyEnabled.ValueBool() || !state.ReadOnlyAPIKeyEnabled.ValueBool() ||
		!state.PingKeyEnabled.ValueBool() || !state.APIKey.IsNull() ||
		!state.ReadOnlyAPIKey.IsNull() || state.PingKey.ValueString() != current.PingKey {
		t.Fatal("existing keys did not survive reconciliation")
	}
}
