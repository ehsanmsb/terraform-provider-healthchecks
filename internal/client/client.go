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
	"path"
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
	ID            string
	Name          string
	APIKey        string
	APIKeyEnabled bool
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
	res, _, err := c.postForm(ctx, "/projects/add/", values, token)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusFound {
		return nil, fmt.Errorf("create project returned status %d", res.StatusCode)
	}

	projectID, err := projectIDFromLocation(res.Header.Get("Location"))
	if err != nil {
		return nil, err
	}

	project, err := c.GetProject(ctx, projectID, "")
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

func (c *Client) GetProject(ctx context.Context, projectID, stateAPIKey string) (*Project, error) {
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

	project := &Project{
		ID:   projectID,
		Name: strings.TrimSpace(doc.Find("div.panel-body.settings-block").First().Text()),
		APIKeyEnabled: doc.Find(`#api-keys tr td:first-child`).FilterFunction(func(i int, s *goquery.Selection) bool {
			return strings.TrimSpace(s.Text()) == "API key"
		}).Length() > 0,
	}

	project.Name = strings.TrimSpace(doc.Find("div.panel-body.settings-block").First().Contents().Not("h2,a").Text())
	if project.Name == "" {
		project.Name = strings.TrimSpace(doc.Find("title").Text())
	}

	if cached := c.cachedProjectAPIKey(projectID); cached != "" {
		project.APIKey = cached
		project.APIKeyEnabled = true
		return project, nil
	}
	if stateAPIKey != "" {
		c.RememberProjectAPIKey(projectID, stateAPIKey)
		project.APIKey = stateAPIKey
		project.APIKeyEnabled = true
		return project, nil
	}

	plaintext := findPlaintextAPIKey(doc)
	if plaintext != "" {
		project.APIKey = plaintext
		project.APIKeyEnabled = true
		c.RememberProjectAPIKey(projectID, plaintext)
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
	return c.GetProject(ctx, projectID, c.cachedProjectAPIKey(projectID))
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
	if res.StatusCode != http.StatusFound {
		return fmt.Errorf("delete project returned status %d", res.StatusCode)
	}
	return nil
}

func (c *Client) EnsureProjectAPIKey(ctx context.Context, projectID string) (string, error) {
	if cached := c.cachedProjectAPIKey(projectID); cached != "" {
		return cached, nil
	}
	if err := c.Login(ctx); err != nil {
		return "", err
	}

	doc, err := c.getDocument(ctx, projectSettingsPath(projectID))
	if err != nil {
		return "", err
	}
	if plaintext := findPlaintextAPIKey(doc); plaintext != "" {
		c.RememberProjectAPIKey(projectID, plaintext)
		return plaintext, nil
	}
	token, err := extractCSRFToken(doc)
	if err != nil {
		return "", err
	}

	values := url.Values{"create_key": {"api_key"}}
	res, body, err := c.postForm(ctx, projectSettingsPath(projectID), values, token)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	key := findKeyCreated(body)
	if key == "" {
		return "", errors.New("project settings did not return a new API key")
	}
	c.RememberProjectAPIKey(projectID, key)
	return key, nil
}

func (c *Client) GetCheck(ctx context.Context, projectID, checkID string) (*Check, error) {
	apiKey, err := c.EnsureProjectAPIKey(ctx, projectID)
	if err != nil {
		return nil, err
	}

	var resp Check
	if err := c.apiJSON(ctx, http.MethodGet, fmt.Sprintf("/api/v3/checks/%s", checkID), apiKey, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) CreateCheck(ctx context.Context, projectID string, payload map[string]any) (*Check, error) {
	apiKey, err := c.EnsureProjectAPIKey(ctx, projectID)
	if err != nil {
		return nil, err
	}
	var wrapper Check
	if err := c.apiJSON(ctx, http.MethodPost, "/api/v3/checks/", apiKey, payload, &wrapper); err != nil {
		return nil, err
	}
	return &wrapper, nil
}

func (c *Client) UpdateCheck(ctx context.Context, projectID, checkID string, payload map[string]any) (*Check, error) {
	apiKey, err := c.EnsureProjectAPIKey(ctx, projectID)
	if err != nil {
		return nil, err
	}
	var check Check
	if err := c.apiJSON(ctx, http.MethodPost, fmt.Sprintf("/api/v3/checks/%s", checkID), apiKey, payload, &check); err != nil {
		return nil, err
	}
	return &check, nil
}

func (c *Client) DeleteCheck(ctx context.Context, projectID, checkID string) error {
	apiKey, err := c.EnsureProjectAPIKey(ctx, projectID)
	if err != nil {
		return err
	}
	if err := c.apiJSON(ctx, http.MethodDelete, fmt.Sprintf("/api/v3/checks/%s", checkID), apiKey, nil, nil); err != nil {
		if errors.Is(err, errNotFound) {
			return nil
		}
		return err
	}
	return nil
}

func (c *Client) ListChannels(ctx context.Context, projectID string) ([]Channel, error) {
	apiKey, err := c.EnsureProjectAPIKey(ctx, projectID)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Channels []Channel `json:"channels"`
	}
	if err := c.apiJSON(ctx, http.MethodGet, "/api/v3/channels/", apiKey, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Channels, nil
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

func (c *Client) GetWebhookIntegration(ctx context.Context, projectID, channelID string) (*Integration, error) {
	if err := c.Login(ctx); err != nil {
		return nil, err
	}
	doc, err := c.getDocument(ctx, fmt.Sprintf("/integrations/%s/edit/", channelID))
	if err != nil {
		return nil, err
	}
	config := map[string]string{}
	for _, field := range []string{"method_down", "url_down", "body_down", "headers_down", "method_up", "url_up", "body_up", "headers_up"} {
		if value, ok := extractNamedFieldValue(doc, field); ok {
			config[field] = value
		}
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
	out.Path = path.Join(c.baseURL.Path, p)
	if !strings.HasPrefix(p, "/") {
		out.Path = path.Join(c.baseURL.Path, "/"+p)
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

func findPlaintextAPIKey(doc *goquery.Document) string {
	row := doc.Find("#api-keys tr").FilterFunction(func(_ int, s *goquery.Selection) bool {
		return strings.TrimSpace(s.Find("td").First().Text()) == "API key"
	}).First()
	if row.Length() == 0 {
		return ""
	}
	code := row.Find("code[data-plaintext]").First()
	if value, ok := code.Attr("data-plaintext"); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func findKeyCreated(body string) string {
	return regexp.MustCompile(`hcw_[A-Za-z0-9]{28}`).FindString(body)
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
