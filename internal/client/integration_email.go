package client

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

func (c *Client) CreateEmailIntegration(ctx context.Context, in Integration) (*Integration, error) {
	id, err := c.createIntegrationChannel(ctx, in, "email", emailConfigToFormValues(in.Config))
	if err != nil {
		return nil, err
	}
	return c.GetEmailIntegration(ctx, in.ProjectID, id)
}

func (c *Client) GetEmailIntegration(ctx context.Context, projectID, channelID string) (*Integration, error) {
	if err := c.Login(ctx); err != nil {
		return nil, err
	}
	doc, err := c.getDocument(ctx, fmt.Sprintf("/integrations/%s/edit/", channelID))
	if err != nil {
		return nil, err
	}

	value, ok := extractNamedFieldValue(doc, "value")
	if !ok || strings.TrimSpace(value) == "" {
		return nil, errors.New("channel did not return an email integration form; verify the channel ID")
	}
	config := map[string]string{"value": value}
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

func emailConfigToFormValues(config map[string]string) url.Values {
	values := url.Values{}
	if value := strings.TrimSpace(config["value"]); value != "" {
		values.Set("value", value)
	}
	if ParseBoolString(config["down"], true) {
		values.Set("down", "on")
	}
	if ParseBoolString(config["up"], true) {
		values.Set("up", "on")
	}
	return values
}

func (c *Client) UpdateEmailIntegration(ctx context.Context, in Integration) (*Integration, error) {
	if err := c.updateIntegrationChannel(ctx, in, emailConfigToFormValues(in.Config)); err != nil {
		return nil, err
	}
	return c.GetEmailIntegration(ctx, in.ProjectID, in.ID)
}
