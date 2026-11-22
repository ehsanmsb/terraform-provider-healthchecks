package client

import (
	"context"
	"fmt"
	"net/url"
)

func (c *Client) CreateNtfyIntegration(ctx context.Context, in Integration) (*Integration, error) {
	id, err := c.createIntegrationChannel(ctx, in, "ntfy", ntfyFormValues(in.Config))
	if err != nil {
		return nil, err
	}
	return c.GetNtfyIntegration(ctx, in.ProjectID, id)
}

func (c *Client) GetNtfyIntegration(ctx context.Context, projectID, channelID string) (*Integration, error) {
	if err := c.Login(ctx); err != nil {
		return nil, err
	}
	doc, err := c.getDocument(ctx, fmt.Sprintf("/integrations/%s/edit/", channelID))
	if err != nil {
		return nil, err
	}
	config := map[string]string{}
	for _, field := range []string{"url", "topic", "token"} {
		value, ok := extractNamedFieldValue(doc, field)
		if !ok {
			return nil, fmt.Errorf("ntfy integration form is missing field %q", field)
		}
		config[field] = value
	}
	for _, field := range []string{"priority", "priority_up"} {
		value, ok := extractNamedSelectValue(doc, field)
		if !ok {
			return nil, fmt.Errorf("ntfy integration form is missing field %q", field)
		}
		config[field] = value
	}
	return &Integration{ID: channelID, ProjectID: projectID, Type: "ntfy", Config: config}, nil
}

func (c *Client) UpdateNtfyIntegration(ctx context.Context, in Integration) (*Integration, error) {
	if err := c.updateIntegrationChannel(ctx, in, ntfyFormValues(in.Config)); err != nil {
		return nil, err
	}
	return c.GetNtfyIntegration(ctx, in.ProjectID, in.ID)
}

func ntfyFormValues(config map[string]string) url.Values {
	values := url.Values{}
	for _, field := range []string{"url", "topic", "priority", "priority_up", "token"} {
		values.Set(field, config[field])
	}
	return values
}
