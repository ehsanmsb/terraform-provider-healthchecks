package client

import "testing"

func TestProjectIDFromLocation(t *testing.T) {
	got, err := projectIDFromLocation("/projects/123e4567-e89b-12d3-a456-426614174000/checks/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "123e4567-e89b-12d3-a456-426614174000" {
		t.Fatalf("unexpected id: %s", got)
	}
}

func TestFindKeyCreated(t *testing.T) {
	key := findKeyCreated(`<div id="key-created-modal">hcw_ABCDEF1234567890123456789012</div>`)
	if key != "hcw_ABCDEF1234567890123456789012" {
		t.Fatalf("unexpected key: %q", key)
	}
}
