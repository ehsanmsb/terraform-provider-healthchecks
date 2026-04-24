package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

var (
	errNotFound       = errors.New("not found")
	errForbidden      = errors.New("forbidden")
	errUnauthorized   = errors.New("unauthorized")
	errUnexpectedAuth = errors.New("unexpected authentication flow")
)

type Config struct {
	BaseURL            string
	Username           string
	Password           string
	InsecureSkipVerify bool
	Timeout            time.Duration
	UserAgent          string
}

type Client struct {
	baseURL   *url.URL
	username  string
	password  string
	http      *http.Client
	userAgent string

	mu             sync.Mutex
	loggedIn       bool
	projectAPIKeys map[string]string
}

type Project struct {
	ID                    string
	Name                  string
	APIKey                string
	APIKeyEnabled         bool
	ReadOnlyAPIKey        string
	ReadOnlyAPIKeyEnabled bool
	PingKey               string
	PingKeyEnabled        bool
}

const (
	projectKeyAPIKey         = "api_key"
	projectKeyReadOnlyAPIKey = "read_only_api_key"
	projectKeyPingKey        = "ping_key"
)

type projectKeyState struct {
	Enabled      bool
	Plaintext    string
	CreateValues url.Values
	RevokeValues url.Values
}

type Check struct {
	ID       string  `json:"uuid"`
	Name     string  `json:"name"`
	Slug     string  `json:"slug"`
	Tags     string  `json:"tags"`
	Desc     string  `json:"desc"`
	Timeout  *int64  `json:"timeout,omitempty"`
	Grace    int64   `json:"grace"`
	Schedule *string `json:"schedule,omitempty"`
	TZ       *string `json:"tz,omitempty"`
	Channels string  `json:"channels,omitempty"`
	PingURL  string  `json:"ping_url,omitempty"`
	Status   string  `json:"status"`
}

type Channel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type Integration struct {
	ID        string
	ProjectID string
	Type      string
	Name      string
	Config    map[string]string
}

type ProjectMember struct {
	ProjectID string
	Email     string
	Role      string
}

func New(cfg Config) (*Client, error) {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://healthchecks.io"
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse base_url: %w", err)
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("create cookie jar: %w", err)
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	return &Client{
		baseURL:  u,
		username: cfg.Username,
		password: cfg.Password,
		http: &http.Client{
			Timeout: timeout,
			Jar:     jar,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: cfg.InsecureSkipVerify},
			},
		},
		userAgent:      cfg.UserAgent,
		projectAPIKeys: map[string]string{},
	}, nil
}

func (c *Client) RememberProjectAPIKey(projectID, apiKey string) {
	if projectID == "" || apiKey == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.projectAPIKeys[projectID] = apiKey
}

func (c *Client) ForgetProjectAPIKey(projectID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.projectAPIKeys, projectID)
}

func (c *Client) Login(ctx context.Context) error {
	c.mu.Lock()
	if c.loggedIn {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	doc, err := c.getDocument(ctx, "/accounts/login/")
	if err != nil {
		return err
	}

	token, err := extractCSRFToken(doc)
	if err != nil {
		return fmt.Errorf("extract login csrf token: %w", err)
	}

	values := url.Values{
		"action":   {"login"},
		"email":    {c.username},
		"password": {c.password},
	}
	res, body, err := c.postForm(ctx, "/accounts/login/", values, token)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		return fmt.Errorf("login failed with status %d", res.StatusCode)
	}

	if strings.Contains(body, "Incorrect email or password") {
		return errUnauthorized
	}
	if strings.Contains(body, "/accounts/login/two_factor/") || strings.Contains(body, "login_webauthn") || strings.Contains(body, "login_totp") {
		return errUnexpectedAuth
	}
	if (res.Request != nil && res.Request.URL != nil && strings.HasSuffix(res.Request.URL.Path, "/accounts/login/")) || strings.Contains(body, `id="login-form"`) {
		return errUnauthorized
	}

	c.mu.Lock()
	c.loggedIn = true
	c.mu.Unlock()
	return nil
}

func (c *Client) CreateProject(ctx context.Context, name string) (*Project, error) {
	if err := c.Login(ctx); err != nil {
		return nil, err
	}

	doc, err := c.getDocument(ctx, "/accounts/profile/")
	if err != nil {
		return nil, err
	}
	token, err := extractCSRFToken(doc)
	if err != nil {
		return nil, err
	}

	values := url.Values{"name": {name}}
	res, body, err := c.postForm(ctx, "/projects/add/", values, token)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	projectID, err := projectIDFromResponse(res, body)
	if err != nil {
		return nil, fmt.Errorf("create project returned status %d: %w", res.StatusCode, err)
	}

	project, err := c.GetProject(ctx, projectID, "", "", "")
	if err != nil {
		return nil, err
	}

	apiKey, err := c.EnsureProjectAPIKey(ctx, projectID)
	if err != nil {
		return nil, err
	}
	project.APIKey = apiKey
	project.APIKeyEnabled = apiKey != ""
	return project, nil
}

func (c *Client) GetProject(ctx context.Context, projectID, stateAPIKey, stateReadOnlyAPIKey, statePingKey string) (*Project, error) {
	if err := c.Login(ctx); err != nil {
		return nil, err
	}

	doc, err := c.getDocument(ctx, projectSettingsPath(projectID))
	if err != nil {
		if errors.Is(err, errNotFound) {
			return nil, errNotFound
		}
		return nil, err
	}

	project := &Project{ID: projectID, Name: strings.TrimSpace(doc.Find("div.panel-body.settings-block").First().Text())}

	project.Name = strings.TrimSpace(doc.Find("div.panel-body.settings-block").First().Contents().Not("h2,a").Text())
	if project.Name == "" {
		project.Name = strings.TrimSpace(doc.Find("title").Text())
	}

	keys := parseProjectKeyStates(doc)

	if key, ok := keys[projectKeyAPIKey]; ok {
		project.APIKeyEnabled = key.Enabled
		if key.Plaintext != "" {
			project.APIKey = key.Plaintext
			c.RememberProjectAPIKey(projectID, key.Plaintext)
		} else if key.Enabled {
			if cached := c.cachedProjectAPIKey(projectID); cached != "" {
				project.APIKey = cached
			} else if stateAPIKey != "" {
				project.APIKey = stateAPIKey
				c.RememberProjectAPIKey(projectID, stateAPIKey)
			}
		}
	} else {
		c.ForgetProjectAPIKey(projectID)
	}

	if key, ok := keys[projectKeyReadOnlyAPIKey]; ok {
		project.ReadOnlyAPIKeyEnabled = key.Enabled
		if key.Plaintext != "" {
			project.ReadOnlyAPIKey = key.Plaintext
		} else if key.Enabled {
			project.ReadOnlyAPIKey = stateReadOnlyAPIKey
		}
	}

	if key, ok := keys[projectKeyPingKey]; ok {
		project.PingKeyEnabled = key.Enabled
		if key.Plaintext != "" {
			project.PingKey = key.Plaintext
		} else if key.Enabled {
			project.PingKey = statePingKey
		}
	}

	return project, nil
}

func (c *Client) UpdateProject(ctx context.Context, projectID, name string) (*Project, error) {
	if err := c.Login(ctx); err != nil {
		return nil, err
	}
	doc, err := c.getDocument(ctx, projectSettingsPath(projectID))
	if err != nil {
		return nil, err
	}
	token, err := extractCSRFToken(doc)
	if err != nil {
		return nil, err
	}

	values := url.Values{
		"set_project_name": {"1"},
		"name":             {name},
	}
	if _, _, err := c.postForm(ctx, projectSettingsPath(projectID), values, token); err != nil {
		return nil, err
	}
	return c.GetProject(ctx, projectID, c.cachedProjectAPIKey(projectID), "", "")
}

func (c *Client) DeleteProject(ctx context.Context, projectID string) error {
	if err := c.Login(ctx); err != nil {
		return err
	}
	doc, err := c.getDocument(ctx, projectSettingsPath(projectID))
	if err != nil {
		if errors.Is(err, errNotFound) {
			return nil
		}
		return err
	}
	token, err := extractCSRFToken(doc)
	if err != nil {
		return err
	}
	res, _, err := c.postForm(ctx, fmt.Sprintf("/projects/%s/remove/", projectID), url.Values{}, token)
	if err != nil {
		if errors.Is(err, errNotFound) {
			return nil
		}
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		return fmt.Errorf("delete project returned status %d", res.StatusCode)
	}
	return nil
}

func (c *Client) EnsureProjectAPIKey(ctx context.Context, projectID string) (string, error) {
	if cached := c.cachedProjectAPIKey(projectID); cached != "" {
		return cached, nil
	}
	key, err := c.ensureProjectKey(ctx, projectID, projectKeyAPIKey)
	if err != nil {
		return "", err
	}
	c.RememberProjectAPIKey(projectID, key)
	return key, nil
}

func (c *Client) EnsureProjectReadOnlyAPIKey(ctx context.Context, projectID string) (string, error) {
	return c.ensureProjectKey(ctx, projectID, projectKeyReadOnlyAPIKey)
}

func (c *Client) EnsureProjectPingKey(ctx context.Context, projectID string) (string, error) {
	return c.ensureProjectKey(ctx, projectID, projectKeyPingKey)
}

func (c *Client) SetProjectKeyEnabled(ctx context.Context, projectID, keyName string, enabled bool) (string, error) {
	if enabled {
		switch keyName {
		case projectKeyAPIKey:
			return c.EnsureProjectAPIKey(ctx, projectID)
		case projectKeyReadOnlyAPIKey:
			return c.EnsureProjectReadOnlyAPIKey(ctx, projectID)
		case projectKeyPingKey:
			return c.EnsureProjectPingKey(ctx, projectID)
		default:
			return "", fmt.Errorf("unsupported project key %q", keyName)
		}
	}

	if err := c.toggleProjectKey(ctx, projectID, keyName, false); err != nil {
		return "", err
	}
	if keyName == projectKeyAPIKey {
		c.ForgetProjectAPIKey(projectID)
	}
	return "", nil
}

func (c *Client) ensureProjectKey(ctx context.Context, projectID, keyName string) (string, error) {
	if keyName == projectKeyAPIKey {
		if cached := c.cachedProjectAPIKey(projectID); cached != "" {
			return cached, nil
		}
	}
	if err := c.Login(ctx); err != nil {
		return "", err
	}

	doc, err := c.getDocument(ctx, projectSettingsPath(projectID))
	if err != nil {
		return "", err
	}
	keys := parseProjectKeyStates(doc)
	keyState, ok := keys[keyName]
	if ok && keyState.Plaintext != "" {
		return keyState.Plaintext, nil
	}
	if keyName == projectKeyAPIKey && ok && keyState.Enabled {
		if cached := c.cachedProjectAPIKey(projectID); cached != "" {
			return cached, nil
		}
	}

	if ok && !keyState.Enabled {
		return c.submitProjectKeyAction(ctx, projectID, doc, keyState.CreateValues)
	}
	if ok && len(keyState.CreateValues) > 0 {
		return c.submitProjectKeyAction(ctx, projectID, doc, keyState.CreateValues)
	}
	if ok && keyState.Enabled && len(keyState.RevokeValues) > 0 {
		if err := c.toggleProjectKey(ctx, projectID, keyName, false); err != nil {
			return "", err
		}
		doc, err = c.getDocument(ctx, projectSettingsPath(projectID))
		if err != nil {
			return "", err
		}
		keys = parseProjectKeyStates(doc)
		keyState = keys[keyName]
		return c.submitProjectKeyAction(ctx, projectID, doc, keyState.CreateValues)
	}

	if keyName == projectKeyAPIKey {
		token, err := extractCSRFToken(doc)
		if err != nil {
			return "", err
		}
		res, body, err := c.postForm(ctx, projectSettingsPath(projectID), url.Values{"create_key": {"api_key"}}, token)
		if err != nil {
			return "", err
		}
		defer res.Body.Close()
		key := findCreatedProjectKey(body)
		if key == "" {
			return "", errors.New("project settings did not return a new API key")
		}
		return key, nil
	}

	return "", fmt.Errorf("project settings did not expose an action for %s", keyName)
}

func (c *Client) submitProjectKeyAction(ctx context.Context, projectID string, doc *goquery.Document, values url.Values) (string, error) {
	if len(values) == 0 {
		return "", errors.New("project settings did not expose a key action")
	}
	token, err := extractCSRFToken(doc)
	if err != nil {
		return "", err
	}
	res, body, err := c.postForm(ctx, projectSettingsPath(projectID), values, token)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	key := findCreatedProjectKey(body)
	if key == "" {
		return "", errors.New("project settings did not return a new API key")
	}
	return key, nil
}

func (c *Client) toggleProjectKey(ctx context.Context, projectID, keyName string, enabled bool) error {
	if err := c.Login(ctx); err != nil {
		return err
	}

	doc, err := c.getDocument(ctx, projectSettingsPath(projectID))
	if err != nil {
		return err
	}
	keys := parseProjectKeyStates(doc)
	keyState, ok := keys[keyName]
	if !ok {
		return fmt.Errorf("project settings did not list key %s", keyName)
	}

	if enabled == keyState.Enabled {
		return nil
	}

	values := keyState.RevokeValues
	if enabled {
		values = keyState.CreateValues
	}
	if len(values) == 0 {
		return fmt.Errorf("project settings did not expose a toggle action for %s", keyName)
	}

	token, err := extractCSRFToken(doc)
	if err != nil {
		return err
	}
	res, _, err := c.postForm(ctx, projectSettingsPath(projectID), values, token)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return nil
}

func (c *Client) GetCheck(ctx context.Context, projectID, checkID string) (*Check, error) {
	var resp Check
	if err := c.withProjectAPIKeyRetry(ctx, projectID, func(apiKey string) error {
		return c.apiJSON(ctx, http.MethodGet, fmt.Sprintf("/api/v3/checks/%s", checkID), apiKey, nil, &resp)
	}); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) CreateCheck(ctx context.Context, projectID string, payload map[string]any) (*Check, error) {
	var wrapper Check
	if err := c.withProjectAPIKeyRetry(ctx, projectID, func(apiKey string) error {
		return c.apiJSON(ctx, http.MethodPost, "/api/v3/checks/", apiKey, payload, &wrapper)
	}); err != nil {
		return nil, err
	}
	return &wrapper, nil
}

func (c *Client) UpdateCheck(ctx context.Context, projectID, checkID string, payload map[string]any) (*Check, error) {
	var check Check
	if err := c.withProjectAPIKeyRetry(ctx, projectID, func(apiKey string) error {
		return c.apiJSON(ctx, http.MethodPost, fmt.Sprintf("/api/v3/checks/%s", checkID), apiKey, payload, &check)
	}); err != nil {
		return nil, err
	}
	return &check, nil
}

func (c *Client) DeleteCheck(ctx context.Context, projectID, checkID string) error {
	if err := c.withProjectAPIKeyRetry(ctx, projectID, func(apiKey string) error {
		return c.apiJSON(ctx, http.MethodDelete, fmt.Sprintf("/api/v3/checks/%s", checkID), apiKey, nil, nil)
	}); err != nil {
		if errors.Is(err, errNotFound) {
			return nil
		}
		return err
	}
	return nil
}

func (c *Client) ListChannels(ctx context.Context, projectID string) ([]Channel, error) {
	var resp struct {
		Channels []Channel `json:"channels"`
	}
	if err := c.withProjectAPIKeyRetry(ctx, projectID, func(apiKey string) error {
		return c.apiJSON(ctx, http.MethodGet, "/api/v3/channels/", apiKey, nil, &resp)
	}); err != nil {
		return nil, err
	}
	return resp.Channels, nil
}

func (c *Client) withProjectAPIKeyRetry(ctx context.Context, projectID string, fn func(apiKey string) error) error {
	apiKey, err := c.EnsureProjectAPIKey(ctx, projectID)
	if err != nil {
		return err
	}

	err = fn(apiKey)
	if !errors.Is(err, errUnauthorized) {
		return err
	}

	c.ForgetProjectAPIKey(projectID)
	apiKey, err = c.EnsureProjectAPIKey(ctx, projectID)
	if err != nil {
		return err
	}

	return fn(apiKey)
}

func (c *Client) CreateWebhookIntegration(ctx context.Context, in Integration) (*Integration, error) {
	if err := c.Login(ctx); err != nil {
		return nil, err
	}

	before, _ := c.ListChannels(ctx, in.ProjectID)
	doc, err := c.getDocument(ctx, fmt.Sprintf("/projects/%s/add_webhook/", in.ProjectID))
	if err != nil {
		return nil, err
	}
	token, err := extractCSRFToken(doc)
	if err != nil {
		return nil, err
	}
	values := url.Values{}
	for k, v := range in.Config {
		values.Set(k, v)
	}
	values.Set("name", in.Name)
	if _, _, err := c.postForm(ctx, fmt.Sprintf("/projects/%s/add_webhook/", in.ProjectID), values, token); err != nil {
		return nil, err
	}

	after, err := c.ListChannels(ctx, in.ProjectID)
	if err != nil {
		return nil, err
	}
	created := diffChannels(before, after)
	if len(created) == 0 {
		return nil, errors.New("could not identify created webhook integration")
	}
	last := created[len(created)-1]
	return c.GetWebhookIntegration(ctx, in.ProjectID, last.ID)
}

func (c *Client) CreateEmailIntegration(ctx context.Context, in Integration) (*Integration, error) {
	if err := c.Login(ctx); err != nil {
		return nil, err
	}

	before, _ := c.ListChannels(ctx, in.ProjectID)
	doc, err := c.getDocument(ctx, fmt.Sprintf("/projects/%s/add_email/", in.ProjectID))
	if err != nil {
		return nil, err
	}
	token, err := extractCSRFToken(doc)
	if err != nil {
		return nil, err
	}

	values := emailConfigToFormValues(in.Config)
	if _, _, err := c.postForm(ctx, fmt.Sprintf("/projects/%s/add_email/", in.ProjectID), values, token); err != nil {
		return nil, err
	}

	after, err := c.ListChannels(ctx, in.ProjectID)
	if err != nil {
		return nil, err
	}
	created := diffChannels(before, after)
	if len(created) == 0 {
		return nil, errors.New("could not identify created email integration")
	}
	last := created[len(created)-1]
	return c.GetEmailIntegration(ctx, in.ProjectID, last.ID)
}

func (c *Client) GetWebhookIntegration(ctx context.Context, projectID, channelID string) (*Integration, error) {
	if err := c.Login(ctx); err != nil {
		return nil, err
	}
	doc, err := c.getDocument(ctx, fmt.Sprintf("/integrations/%s/edit/", channelID))
	if err != nil {
		return nil, err
	}
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
	return &Integration{
		ID:        channelID,
		ProjectID: projectID,
		Type:      "webhook",
		Name:      name,
		Config:    config,
	}, nil
}

func (c *Client) GetEmailIntegration(ctx context.Context, projectID, channelID string) (*Integration, error) {
	if err := c.Login(ctx); err != nil {
		return nil, err
	}
	doc, err := c.getDocument(ctx, fmt.Sprintf("/integrations/%s/edit/", channelID))
	if err != nil {
		return nil, err
	}

	config := map[string]string{}
	if value, ok := extractNamedFieldValue(doc, "value"); ok && strings.TrimSpace(value) != "" {
		config["value"] = value
	}
	config["up"] = boolString(extractNamedCheckboxValue(doc, "up"))
	config["down"] = boolString(extractNamedCheckboxValue(doc, "down"))

	return &Integration{
		ID:        channelID,
		ProjectID: projectID,
		Type:      "email",
		Name:      "",
		Config:    config,
	}, nil
}

func (c *Client) UpdateWebhookIntegration(ctx context.Context, in Integration) (*Integration, error) {
	if err := c.Login(ctx); err != nil {
		return nil, err
	}
	doc, err := c.getDocument(ctx, fmt.Sprintf("/integrations/%s/edit/", in.ID))
	if err != nil {
		return nil, err
	}
	token, err := extractCSRFToken(doc)
	if err != nil {
		return nil, err
	}
	values := url.Values{}
	for k, v := range in.Config {
		values.Set(k, v)
	}
	values.Set("name", in.Name)
	if _, _, err := c.postForm(ctx, fmt.Sprintf("/integrations/%s/edit/", in.ID), values, token); err != nil {
		return nil, err
	}
	return c.GetWebhookIntegration(ctx, in.ProjectID, in.ID)
}

func (c *Client) UpdateEmailIntegration(ctx context.Context, in Integration) (*Integration, error) {
	if err := c.Login(ctx); err != nil {
		return nil, err
	}
	doc, err := c.getDocument(ctx, fmt.Sprintf("/integrations/%s/edit/", in.ID))
	if err != nil {
		return nil, err
	}
	token, err := extractCSRFToken(doc)
	if err != nil {
		return nil, err
	}

	values := emailConfigToFormValues(in.Config)
	if _, _, err := c.postForm(ctx, fmt.Sprintf("/integrations/%s/edit/", in.ID), values, token); err != nil {
		return nil, err
	}
	return c.GetEmailIntegration(ctx, in.ProjectID, in.ID)
}

func (c *Client) DeleteIntegration(ctx context.Context, integrationID string) error {
	if err := c.Login(ctx); err != nil {
		return err
	}
	doc, err := c.getDocument(ctx, "/accounts/profile/")
	if err != nil {
		return err
	}
	token, err := extractCSRFToken(doc)
	if err != nil {
		return err
	}
	if _, _, err := c.postForm(ctx, fmt.Sprintf("/integrations/%s/remove/", integrationID), url.Values{}, token); err != nil {
		if errors.Is(err, errNotFound) {
			return nil
		}
		return err
	}
	return nil
}

func (c *Client) GetProjectMember(ctx context.Context, projectID, email string) (*ProjectMember, error) {
	if err := c.Login(ctx); err != nil {
		return nil, err
	}
	doc, err := c.getDocument(ctx, projectSettingsPath(projectID))
	if err != nil {
		return nil, err
	}

	var role string
	found := false
	doc.Find("#team-table tr").Each(func(_ int, tr *goquery.Selection) {
		cells := tr.Find("td")
		if cells.Length() < 2 {
			return
		}
		memberEmail := strings.TrimSpace(cells.Eq(0).Text())
		if !strings.EqualFold(memberEmail, email) {
			return
		}
		role = strings.TrimSpace(cells.Eq(1).Text())
		found = true
	})
	if !found {
		return nil, errNotFound
	}
	return &ProjectMember{ProjectID: projectID, Email: strings.ToLower(email), Role: normalizeRoleLabel(role)}, nil
}

func (c *Client) UpsertProjectMember(ctx context.Context, projectID, email, role string, existing *ProjectMember) (*ProjectMember, error) {
	if existing != nil && existing.Role == role && strings.EqualFold(existing.Email, email) {
		return existing, nil
	}
	if existing != nil {
		if err := c.RemoveProjectMember(ctx, projectID, existing.Email); err != nil {
			return nil, err
		}
	}
	if err := c.Login(ctx); err != nil {
		return nil, err
	}
	doc, err := c.getDocument(ctx, projectSettingsPath(projectID))
	if err != nil {
		return nil, err
	}
	token, err := extractCSRFToken(doc)
	if err != nil {
		return nil, err
	}
	values := url.Values{
		"invite_team_member": {"1"},
		"email":              {strings.ToLower(email)},
		"role":               {role},
	}
	if _, _, err := c.postForm(ctx, projectSettingsPath(projectID), values, token); err != nil {
		return nil, err
	}
	return c.GetProjectMember(ctx, projectID, email)
}

func (c *Client) RemoveProjectMember(ctx context.Context, projectID, email string) error {
	if err := c.Login(ctx); err != nil {
		return err
	}
	doc, err := c.getDocument(ctx, projectSettingsPath(projectID))
	if err != nil {
		if errors.Is(err, errNotFound) {
			return nil
		}
		return err
	}
	token, err := extractCSRFToken(doc)
	if err != nil {
		return err
	}
	values := url.Values{
		"remove_team_member": {"1"},
		"email":              {strings.ToLower(email)},
	}
	if _, _, err := c.postForm(ctx, projectSettingsPath(projectID), values, token); err != nil {
		if errors.Is(err, errNotFound) {
			return nil
		}
		return err
	}
	return nil
}

func (c *Client) cachedProjectAPIKey(projectID string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.projectAPIKeys[projectID]
}

func (c *Client) getDocument(ctx context.Context, p string) (*goquery.Document, error) {
	req, err := c.newRequest(ctx, http.MethodGet, p, nil)
	if err != nil {
		return nil, err
	}
	res, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return nil, errNotFound
	}
	if res.StatusCode == http.StatusForbidden {
		return nil, errForbidden
	}
	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("GET %s returned status %d", p, res.StatusCode)
	}
	return goquery.NewDocumentFromReader(res.Body)
}

func (c *Client) postForm(ctx context.Context, p string, values url.Values, csrfToken string) (*http.Response, string, error) {
	if csrfToken != "" {
		values.Set("csrfmiddlewaretoken", csrfToken)
	}
	req, err := c.newRequest(ctx, http.MethodPost, p, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", c.resolveURL(p).String())
	res, err := c.do(req)
	if err != nil {
		return nil, "", err
	}
	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		res.Body.Close()
		return nil, "", err
	}
	res.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	return res, string(bodyBytes), nil
}

func (c *Client) apiJSON(ctx context.Context, method, p, apiKey string, payload any, target any) error {
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := c.newRequest(ctx, method, p, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Api-Key", apiKey)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		return errNotFound
	}
	if res.StatusCode == http.StatusForbidden {
		return errForbidden
	}
	if res.StatusCode == http.StatusUnauthorized {
		c.mu.Lock()
		for id, key := range c.projectAPIKeys {
			if key == apiKey {
				delete(c.projectAPIKeys, id)
			}
		}
		c.mu.Unlock()
		return errUnauthorized
	}
	if res.StatusCode >= 400 {
		raw, _ := io.ReadAll(res.Body)
		return fmt.Errorf("api %s %s returned status %d: %s", method, p, res.StatusCode, strings.TrimSpace(string(raw)))
	}
	if target == nil {
		return nil
	}
	return json.NewDecoder(res.Body).Decode(target)
}

func (c *Client) newRequest(ctx context.Context, method, p string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.resolveURL(p).String(), body)
	if err != nil {
		return nil, err
	}
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	return req, nil
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	var res *http.Response
	var err error
	backoff := 500 * time.Millisecond
	for attempt := 0; attempt < 4; attempt++ {
		res, err = c.http.Do(req)
		if err != nil {
			return nil, err
		}
		if res.StatusCode != http.StatusTooManyRequests || attempt == 3 {
			return res, nil
		}
		io.Copy(io.Discard, res.Body)
		res.Body.Close()
		time.Sleep(backoff)
		backoff *= 2
	}
	return res, err
}

func (c *Client) resolveURL(p string) *url.URL {
	out := *c.baseURL
	basePath := strings.TrimRight(c.baseURL.Path, "/")
	reqPath := p
	if !strings.HasPrefix(reqPath, "/") {
		reqPath = "/" + reqPath
	}
	out.Path = basePath + reqPath
	if out.Path == "" {
		out.Path = "/"
	}
	return &out
}

func projectSettingsPath(projectID string) string {
	return fmt.Sprintf("/projects/%s/settings/", projectID)
}

func projectIDFromLocation(location string) (string, error) {
	matches := regexp.MustCompile(`/projects/([0-9a-f-]+)/checks/?`).FindStringSubmatch(location)
	if len(matches) != 2 {
		return "", fmt.Errorf("could not parse project id from redirect %q", location)
	}
	return matches[1], nil
}

func projectIDFromBody(body string) (string, error) {
	matches := regexp.MustCompile(`/projects/([0-9a-f-]+)/`).FindStringSubmatch(body)
	if len(matches) == 2 {
		return matches[1], nil
	}

	return "", errors.New("could not determine project id from response body")
}

func projectIDFromResponse(res *http.Response, body string) (string, error) {
	if location := res.Header.Get("Location"); location != "" {
		if id, err := projectIDFromLocation(location); err == nil {
			return id, nil
		}
	}

	if res.Request != nil && res.Request.URL != nil {
		if id, err := projectIDFromLocation(res.Request.URL.String()); err == nil {
			return id, nil
		}
	}

	if body != "" {
		if id, err := projectIDFromBody(body); err == nil {
			return id, nil
		}
	}

	return "", errors.New("could not determine project id from response")
}

func extractCSRFToken(doc *goquery.Document) (string, error) {
	if token, ok := extractNamedFieldValue(doc, "csrfmiddlewaretoken"); ok && token != "" {
		return token, nil
	}
	return "", errors.New("csrf token not found")
}

func extractNamedFieldValue(doc *goquery.Document, name string) (string, bool) {
	selection := doc.Find(fmt.Sprintf(`[name="%s"]`, name)).First()
	if selection.Length() == 0 {
		return "", false
	}
	if value, ok := selection.Attr("value"); ok {
		return value, true
	}
	return strings.TrimSpace(selection.Text()), true
}

func extractNamedCheckboxValue(doc *goquery.Document, name string) bool {
	selection := doc.Find(fmt.Sprintf(`[name="%s"]`, name)).First()
	if selection.Length() == 0 {
		return false
	}
	_, ok := selection.Attr("checked")
	return ok
}

func extractNamedSelectValue(doc *goquery.Document, name string) (string, bool) {
	selection := doc.Find(fmt.Sprintf(`select[name="%s"]`, name)).First()
	if selection.Length() == 0 {
		return "", false
	}

	selected := selection.Find("option[selected]").First()
	if selected.Length() == 0 {
		selected = selection.Find("option").First()
	}
	if selected.Length() == 0 {
		return "", false
	}

	if value, ok := selected.Attr("value"); ok {
		return strings.TrimSpace(value), true
	}
	return strings.TrimSpace(selected.Text()), true
}

func parseProjectKeyStates(doc *goquery.Document) map[string]projectKeyState {
	out := map[string]projectKeyState{}

	doc.Find("#api-keys tr").Each(func(_ int, row *goquery.Selection) {
		name := normalizeProjectKeyLabel(strings.TrimSpace(row.Find("td").First().Text()))
		if name == "" {
			return
		}

		state := projectKeyState{}
		if value, ok := row.Find("code[data-plaintext]").First().Attr("data-plaintext"); ok {
			state.Plaintext = strings.TrimSpace(value)
		}

		if keyType, ok := row.Find("[data-create-key]").First().Attr("data-create-key"); ok && keyType != "" {
			state.CreateValues = url.Values{"create_key": {strings.TrimSpace(keyType)}}
		}
		if keyType, ok := row.Find("[data-revoke-key]").First().Attr("data-revoke-key"); ok && keyType != "" {
			state.RevokeValues = url.Values{"revoke_key": {strings.TrimSpace(keyType)}}
		}

		row.Find("form").Each(func(_ int, form *goquery.Selection) {
			values := formValues(form)
			if values.Get("create_key") != "" {
				state.CreateValues = values
			}
			if values.Get("revoke_key") != "" {
				state.RevokeValues = values
			}
		})

		switch {
		case len(state.RevokeValues) > 0:
			state.Enabled = true
		case len(state.CreateValues) > 0:
			state.Enabled = false
		case row.Find(".not-set").Length() > 0:
			state.Enabled = false
		case state.Plaintext != "":
			state.Enabled = true
		case strings.TrimSpace(row.Find("td").Eq(1).Text()) != "":
			state.Enabled = true
		}

		out[name] = state
	})

	return out
}

func formValues(form *goquery.Selection) url.Values {
	values := url.Values{}

	form.Find("input[name], button[name], textarea[name], select[name]").Each(func(_ int, field *goquery.Selection) {
		name, ok := field.Attr("name")
		if !ok || name == "" || name == "csrfmiddlewaretoken" {
			return
		}

		value, _ := field.Attr("value")
		tag := goquery.NodeName(field)
		if tag == "textarea" && value == "" {
			value = strings.TrimSpace(field.Text())
		}
		if tag == "select" && value == "" {
			field.Find("option[selected]").Each(func(_ int, option *goquery.Selection) {
				if value == "" {
					value, _ = option.Attr("value")
				}
			})
		}
		values.Set(name, value)
	})

	return values
}

func normalizeProjectKeyLabel(label string) string {
	normalized := strings.ToLower(strings.TrimSpace(label))

	switch normalized {
	case "api key":
		return projectKeyAPIKey
	case "api key (read-only)", "read-only api key", "api key readonly":
		return projectKeyReadOnlyAPIKey
	case "ping key":
		return projectKeyPingKey
	default:
		return ""
	}
}

func findCreatedProjectKey(body string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err == nil {
		if value, ok := doc.Find(`#key-created-modal input[readonly]`).First().Attr("value"); ok {
			return strings.TrimSpace(value)
		}
		if value, ok := doc.Find("#key-created-modal code[data-plaintext]").First().Attr("data-plaintext"); ok {
			return strings.TrimSpace(value)
		}
		text := strings.TrimSpace(doc.Find("#key-created-modal").First().Text())
		if text != "" {
			if match := regexp.MustCompile(`\b(?:hc[wr]_[A-Za-z0-9]{28}|[A-Za-z0-9]{20,64})\b`).FindString(text); match != "" {
				return match
			}
		}
	}
	return ""
}

func diffChannels(before, after []Channel) []Channel {
	known := map[string]struct{}{}
	for _, ch := range before {
		known[ch.ID] = struct{}{}
	}
	var out []Channel
	for _, ch := range after {
		if _, ok := known[ch.ID]; !ok {
			out = append(out, ch)
		}
	}
	slices.SortFunc(out, func(a, b Channel) int {
		return strings.Compare(a.ID, b.ID)
	})
	return out
}

func normalizeRoleLabel(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "read-only":
		return "r"
	case "manager":
		return "m"
	case "member":
		return "w"
	default:
		return role
	}
}

func emailConfigToFormValues(config map[string]string) url.Values {
	values := url.Values{}
	if value := strings.TrimSpace(config["value"]); value != "" {
		values.Set("value", value)
	}
	if parseBoolString(config["down"], true) {
		values.Set("down", "on")
	}
	if parseBoolString(config["up"], true) {
		values.Set("up", "on")
	}
	return values
}

func parseBoolString(v string, defaultValue bool) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "":
		return defaultValue
	case "1", "t", "true", "yes", "on":
		return true
	case "0", "f", "false", "no", "off":
		return false
	default:
		return defaultValue
	}
}

func boolString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
