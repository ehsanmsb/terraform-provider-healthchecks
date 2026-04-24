package client

import (
	"net/http"
	"net/url"
	"testing"
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
	key := findKeyCreated(`<div id="key-created-modal">hcw_ABCDEF1234567890123456789012</div>`)
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
