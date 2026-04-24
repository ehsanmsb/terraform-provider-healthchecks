package main

import (
	"os"
	"testing"
)

func TestAccEnv(t *testing.T) {
	if os.Getenv("HEALTHCHECKS_BASE_URL") == "" || os.Getenv("HEALTHCHECKS_USERNAME") == "" || os.Getenv("HEALTHCHECKS_PASSWORD") == "" {
		t.Skip("acceptance tests require HEALTHCHECKS_BASE_URL, HEALTHCHECKS_USERNAME, and HEALTHCHECKS_PASSWORD")
	}
}
