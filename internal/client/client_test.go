package client

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

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
	key := findCreatedProjectKey(`<div id="key-created-modal">hcw_ABCDEF1234567890123456789012</div>`)
	if key != "hcw_ABCDEF1234567890123456789012" {
		t.Fatalf("unexpected key: %q", key)
	}
}

func TestProjectIDFromResponseUsesFinalURL(t *testing.T) {
	u, err := url.Parse("https://healthchecks.example.com/projects/123e4567-e89b-12d3-a456-426614174000/checks/")
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}

	res := &http.Response{
		StatusCode: http.StatusOK,
		Request: &http.Request{
			URL: u,
		},
		Header: http.Header{},
	}

	got, err := projectIDFromResponse(res, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "123e4567-e89b-12d3-a456-426614174000" {
		t.Fatalf("unexpected id: %s", got)
	}
}

func TestProjectIDFromResponseUsesBody(t *testing.T) {
	res := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{},
	}

	body := `<a href="/projects/123e4567-e89b-12d3-a456-426614174000/settings/">settings</a>`

	got, err := projectIDFromResponse(res, body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "123e4567-e89b-12d3-a456-426614174000" {
		t.Fatalf("unexpected id: %s", got)
	}
}

func TestResolveURLPreservesTrailingSlash(t *testing.T) {
	c, err := New(Config{BaseURL: "http://localhost:8000/"})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	got := c.resolveURL("/accounts/login/").String()
	if got != "http://localhost:8000/accounts/login/" {
		t.Fatalf("unexpected url: %s", got)
	}
}

func TestParseProjectKeyStates(t *testing.T) {
	doc := docFromString(t, `
		<table id="api-keys">
			<tr>
				<td>API key</td>
				<td><code data-plaintext="hcw_ABCDEFGHIJKLMNOPQRSTUVWXYZ12"></code></td>
				<td><a data-revoke-key="api_key">Revoke</a></td>
			</tr>
			<tr>
				<td>API key (read-only)</td>
				<td><span class="not-set">not set</span></td>
				<td><a data-create-key="api_key_readonly">Create</a></td>
			</tr>
			<tr>
				<td>Ping key</td>
				<td>1fj9XWM6Ns8vLGTmnPGk9g</td>
				<td><a data-revoke-key="ping_key">Revoke</a></td>
			</tr>
		</table>
	`)

	got := parseProjectKeyStates(doc)

	if !got[projectKeyAPIKey].Enabled {
		t.Fatalf("expected api key enabled")
	}
	if got[projectKeyAPIKey].Plaintext != "hcw_ABCDEFGHIJKLMNOPQRSTUVWXYZ12" {
		t.Fatalf("unexpected plaintext: %q", got[projectKeyAPIKey].Plaintext)
	}
	if got[projectKeyReadOnlyAPIKey].Enabled {
		t.Fatalf("expected read-only api key disabled")
	}
	if got[projectKeyReadOnlyAPIKey].CreateValues.Get("create_key") != "api_key_readonly" {
		t.Fatalf("unexpected read-only create values: %#v", got[projectKeyReadOnlyAPIKey].CreateValues)
	}
	if !got[projectKeyPingKey].Enabled {
		t.Fatalf("expected ping key enabled")
	}
	if got[projectKeyPingKey].RevokeValues.Get("revoke_key") != "ping_key" {
		t.Fatalf("unexpected ping revoke values: %#v", got[projectKeyPingKey].RevokeValues)
	}
}

func TestFindCreatedProjectKeyPingKey(t *testing.T) {
	body := `<div id="key-created-modal"><input readonly value="1fj9XWM6Ns8vLGTmnPGk9g" /></div>`
	got := findCreatedProjectKey(body)
	if got != "1fj9XWM6Ns8vLGTmnPGk9g" {
		t.Fatalf("unexpected key: %q", got)
	}
}

func TestFindCreatedProjectKeyIgnoresCSRFToken(t *testing.T) {
	body := `
		<input type="hidden" name="csrfmiddlewaretoken" value="DHl48M7lQmV2prjCZTfz1pNwJOLWkfL1EEsh4zfgHzbKxNeALJGKVQ7G9uylTUW9">
		<div id="key-created-modal">
			<input type="text" class="form-control" value="hcw_UapC9r2P2GUaX8YegBqUoBMDBX8C" readonly />
		</div>
	`
	got := findCreatedProjectKey(body)
	if got != "hcw_UapC9r2P2GUaX8YegBqUoBMDBX8C" {
		t.Fatalf("unexpected key: %q", got)
	}
}

func TestEmailConfigToFormValues(t *testing.T) {
	values := emailConfigToFormValues(map[string]string{
		"value": "ops@example.com",
		"up":    "false",
		"down":  "true",
	})

	if values.Get("value") != "ops@example.com" {
		t.Fatalf("unexpected value: %q", values.Get("value"))
	}
	if values.Get("down") != "on" {
		t.Fatalf("expected down checkbox to be set")
	}
	if values.Get("up") != "" {
		t.Fatalf("expected up checkbox to be omitted")
	}
}

func TestWebhookConfigToFormValues(t *testing.T) {
	values := webhookConfigToFormValues("Primary Webhook", map[string]string{
		"method_down":  "POST",
		"url_down":     "https://example.com/down",
		"headers_down": "X-Sample-Header: $NAME has gone down",
		"method_up":    "POST",
		"url_up":       "https://example.com/up",
		"headers_up":   "X-Sample-Header: $NAME has recovered",
	})

	if values.Get("name") != "Primary Webhook" {
		t.Fatalf("unexpected name: %q", values.Get("name"))
	}
	if values.Get("headers_down") != "X-Sample-Header: $NAME has gone down" {
		t.Fatalf("unexpected down headers: %q", values.Get("headers_down"))
	}
	if values.Get("headers_up") != "X-Sample-Header: $NAME has recovered" {
		t.Fatalf("unexpected up headers: %q", values.Get("headers_up"))
	}
}

func TestGetWebhookIntegrationParsesNameAndHeaders(t *testing.T) {
	doc := docFromString(t, `
		<form>
			<input type="text" name="name" value="Primary Webhook">
			<select name="method_down"><option value="POST" selected>POST</option></select>
			<input type="url" name="url_down" value="https://example.com/down">
			<textarea name="headers_down">X-Sample-Header: $NAME has gone down</textarea>
			<select name="method_up"><option value="POST" selected>POST</option></select>
			<input type="url" name="url_up" value="https://example.com/up">
			<textarea name="headers_up">X-Sample-Header: $NAME has recovered</textarea>
		</form>
	`)

	config := map[string]string{}
	for _, field := range []string{"url_down", "body_down", "headers_down", "url_up", "body_up", "headers_up"} {
		if value, ok := extractNamedFieldValue(doc, field); ok && strings.TrimSpace(value) != "" {
			config[field] = value
		}
	}
	if value, ok := extractNamedSelectValue(doc, "method_down"); ok {
		config["method_down"] = value
	}
	if value, ok := extractNamedSelectValue(doc, "method_up"); ok {
		config["method_up"] = value
	}
	name, _ := extractNamedFieldValue(doc, "name")

	if name != "Primary Webhook" {
		t.Fatalf("unexpected name: %q", name)
	}
	if config["headers_down"] != "X-Sample-Header: $NAME has gone down" {
		t.Fatalf("unexpected down headers: %q", config["headers_down"])
	}
	if config["headers_up"] != "X-Sample-Header: $NAME has recovered" {
		t.Fatalf("unexpected up headers: %q", config["headers_up"])
	}
}

func TestParseBoolString(t *testing.T) {
	cases := []struct {
		input        string
		defaultValue bool
		want         bool
	}{
		{input: "true", defaultValue: false, want: true},
		{input: "false", defaultValue: true, want: false},
		{input: "", defaultValue: true, want: true},
		{input: "", defaultValue: false, want: false},
		{input: "on", defaultValue: false, want: true},
		{input: "off", defaultValue: true, want: false},
	}

	for _, tc := range cases {
		if got := parseBoolString(tc.input, tc.defaultValue); got != tc.want {
			t.Fatalf("parseBoolString(%q, %t) = %t, want %t", tc.input, tc.defaultValue, got, tc.want)
		}
	}
}

func docFromString(t *testing.T, body string) *goquery.Document {
	t.Helper()

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		t.Fatalf("new document: %v", err)
	}

	return doc
}
