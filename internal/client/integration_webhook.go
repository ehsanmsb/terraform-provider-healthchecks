package client

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

func (c *Client) CreateWebhookIntegration(ctx context.Context, in Integration) (*Integration, error) {
	id, err := c.createIntegrationChannel(ctx, in, "webhook", webhookConfigToFormValues(in.Name, in.Config))
	if err != nil {
		return nil, err
	}
	return c.GetWebhookIntegration(ctx, in.ProjectID, id)
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

func webhookConfigToFormValues(name string, config map[string]string) url.Values {
	values := url.Values{}
	for key, value := range config {
		values.Set(key, value)
	}
	values.Set("name", name)
	return values
}

func (c *Client) UpdateWebhookIntegration(ctx context.Context, in Integration) (*Integration, error) {
	if err := c.updateIntegrationChannel(ctx, in, webhookConfigToFormValues(in.Name, in.Config)); err != nil {
		return nil, err
	}
	return c.GetWebhookIntegration(ctx, in.ProjectID, in.ID)
}
