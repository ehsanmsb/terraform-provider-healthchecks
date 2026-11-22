package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ehsanmsb/terraform-provider-healthchecks/internal/client"
)

func TestAccProvider(t *testing.T) {
	if os.Getenv("TF_ACC") != "1" {
		t.Skip("set TF_ACC=1 to test against a disposable Healthchecks instance")
	}

	env := []string{
		"TF_IN_AUTOMATION=1",
		"TF_INPUT=0",
		"TF_WORKSPACE=default",
		"TF_DATA_DIR=.terraform",
		"TF_CLI_ARGS=",
		"TF_LOG=",
		"TF_LOG_CORE=",
		"TF_LOG_PROVIDER=",
		"TF_LOG_PATH=",
	}
	for _, command := range []string{"apply", "destroy", "import", "output", "plan", "state"} {
		env = append(env, "TF_CLI_ARGS_"+command+"=")
	}
	for _, input := range []struct{ env, variable string }{
		{"HEALTHCHECKS_BASE_URL", "base_url"},
		{"HEALTHCHECKS_USERNAME", "username"},
		{"HEALTHCHECKS_PASSWORD", "password"},
		{"HEALTHCHECKS_TEST_MEMBER_EMAIL", "member_email"},
	} {
		value := os.Getenv(input.env)
		if value == "" {
			t.Fatalf("%s is required when TF_ACC=1", input.env)
		}
		env = append(env, "TF_VAR_"+input.variable+"="+value)
	}
	username := os.Getenv("HEALTHCHECKS_USERNAME")
	password := os.Getenv("HEALTHCHECKS_PASSWORD")
	member := strings.ToUpper(os.Getenv("HEALTHCHECKS_TEST_MEMBER_EMAIL"))
	env = append(env, "TF_VAR_member_email="+member)
	if strings.EqualFold(username, member) {
		t.Fatal("the test member must be a separate Healthchecks account")
	}
	terraform, err := exec.LookPath("terraform")
	if err != nil {
		t.Fatal(err)
	}

	binDir := t.TempDir()
	binary := filepath.Join(binDir, "terraform-provider-healthchecks")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.Command("go", "build", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build provider: %v\n%s", err, output)
	}

	config := filepath.Join(binDir, "terraform.tfrc")
	writeAccFile(t, config, []byte(fmt.Sprintf(`provider_installation {
  dev_overrides {
    "ehsanmsb/healthchecks" = %q
  }
  direct {}
}
`, filepath.ToSlash(binDir))))
	projectName := fmt.Sprintf("Terraform Acceptance %d", time.Now().UnixNano())
	env = append(env,
		"TF_CLI_CONFIG_FILE="+config,
		"TF_VAR_project_name="+projectName,
		"TF_VAR_keys_enabled=true",
		"TF_VAR_project_key=",
		"TF_VAR_updated=false",
		"TF_VAR_importing=false",
	)
	fixture, err := os.ReadFile("testdata/acceptance/main.tf")
	if err != nil {
		t.Fatal(err)
	}
	managed := t.TempDir()
	writeAccFile(t, filepath.Join(managed, "main.tf"), fixture)

	secrets := []string{password}
	run := func(dir string, args ...string) []byte {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, terraform, args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), env...)
		output, err := cmd.Output()
		if err != nil {
			diagnostic := string(output)
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				diagnostic += "\n" + string(exit.Stderr)
			}
			for _, secret := range secrets {
				diagnostic = strings.ReplaceAll(diagnostic, secret, "[redacted]")
			}
			t.Fatalf("terraform %s: %v\n%s", args[0], err, diagnostic)
		}
		return output
	}
	apply := func(args ...string) {
		t.Helper()
		run(managed, append([]string{"apply", "-no-color", "-auto-approve"}, args...)...)
	}
	destroy := func() {
		t.Helper()
		run(managed, "destroy", "-no-color", "-auto-approve")
	}
	t.Cleanup(func() {
		if t.Failed() {
			if _, err := os.Stat(filepath.Join(managed, "terraform.tfstate")); err == nil {
				destroy()
			}
		}
	})

	readKeys := func() map[string]string {
		t.Helper()
		var keys map[string]string
		if err := json.Unmarshal(run(managed, "output", "-json", "keys"), &keys); err != nil {
			t.Fatal("cannot decode project keys from Terraform output")
		}
		for _, key := range keys {
			if key != "" {
				secrets = append(secrets, key)
			}
		}
		return keys
	}
	var previousKeys map[string]string
	for _, enabled := range []bool{true, false, true} {
		t.Logf("Apply project with keys enabled = %t", enabled)
		args := []string{"-target=healthchecks_project.test", fmt.Sprintf("-var=keys_enabled=%t", enabled)}
		apply(args...)
		keys := readKeys()
		for _, name := range []string{"read_write", "read_only", "ping"} {
			if (keys[name] != "") != enabled {
				t.Fatalf("%s key does not match enabled = %t", name, enabled)
			}
		}
		if enabled {
			for name, previous := range previousKeys {
				if keys[name] == previous {
					t.Fatalf("%s key was reused after re-enabling it", name)
				}
			}
			previousKeys = keys
			env = append(env, "TF_VAR_project_key="+keys["read_write"])
		}
	}

	t.Log("Create the check, all three integrations and a member, then verify attachment through the check data source")
	apply()
	type resources struct {
		Project     string
		ProjectName string `json:"project_name"`
		KeysEnabled bool   `json:"keys_enabled"`
		WebhookName string `json:"webhook_name"`
		Ntfy        string
		NtfyTopic   string `json:"ntfy_topic"`
		Check       string
		Webhook     string
		Email       string
		Channels    []string
	}
	readResources := func() resources {
		t.Helper()
		var result resources
		if err := json.Unmarshal(run(managed, "output", "-json", "resources"), &result); err != nil {
			t.Fatal(err)
		}
		slices.Sort(result.Channels)
		return result
	}
	created := readResources()
	if created.ProjectName != projectName || !created.KeysEnabled || created.WebhookName != "Acceptance Webhook" || created.NtfyTopic != "acceptance" {
		t.Fatal("data sources did not return the expected project and webhook metadata")
	}
	wantChannels := []string{created.Webhook, created.Email, created.Ntfy}
	slices.Sort(wantChannels)
	if created.Project == "" || created.Check == "" || created.Webhook == "" || created.Email == "" ||
		created.Ntfy == "" || created.Webhook == created.Email || !slices.Equal(created.Channels, wantChannels) {
		t.Fatal("the check was not attached to the expected integrations")
	}

	t.Log("Update all resource types and detach all channels")
	env = append(env, "TF_VAR_updated=true")
	apply()
	updated := readResources()
	if updated.ProjectName != projectName+" updated" || !updated.KeysEnabled || updated.WebhookName != "Acceptance Webhook Updated" || updated.NtfyTopic != "acceptance-updated" {
		t.Fatal("data sources did not refresh the updated metadata")
	}
	if updated.Project != created.Project || updated.Check != created.Check ||
		updated.Webhook != created.Webhook || updated.Email != created.Email || updated.Ntfy != created.Ntfy || len(updated.Channels) != 0 {
		t.Fatal("the update should preserve resource IDs and detach all channels")
	}
	run(managed, "plan", "-no-color", "-detailed-exitcode")

	t.Log("Import every resource into a separate state and verify a no-op plan")
	imported := t.TempDir()
	writeAccFile(t, filepath.Join(imported, "main.tf"), fixture)
	env = append(env, "TF_VAR_importing=true")
	for _, entry := range []struct{ resource, id string }{
		{"healthchecks_project.test", created.Project},
		{"healthchecks_integration.webhook", created.Project + "/webhook/" + created.Webhook},
		{"healthchecks_integration.email", created.Project + "/email/" + created.Email},
		{"healthchecks_integration.ntfy", created.Project + "/ntfy/" + created.Ntfy},
		{"healthchecks_check.test", created.Project + "/" + created.Check},
		{"healthchecks_project_member.test", created.Project + "/" + member},
	} {
		run(imported, "import", "-no-color", entry.resource, entry.id)
	}
	run(imported, "plan", "-no-color", "-detailed-exitcode")

	t.Log("Destroy from the original state and verify the project is gone")
	env = append(env, "TF_VAR_importing=false")
	destroy()
	if state := strings.TrimSpace(string(run(managed, "state", "list"))); state != "" {
		t.Fatal("Terraform state still contains resources after destroy")
	}
	account, err := client.New(client.Config{
		BaseURL:  os.Getenv("HEALTHCHECKS_BASE_URL"),
		Username: username,
		Password: password,
		Timeout:  30 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := account.GetProject(context.Background(), created.Project, "", "", ""); !errors.Is(err, client.ErrNotFound()) {
		t.Fatalf("expected the destroyed project to be absent, got %v", err)
	}
}

func writeAccFile(t *testing.T, path string, contents []byte) {
	t.Helper()
	if err := os.WriteFile(path, contents, 0600); err != nil {
		t.Fatal(err)
	}
}
