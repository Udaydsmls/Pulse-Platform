package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"go.uber.org/zap"
)

// EmailSender defines the interface for sending email notifications.
type EmailSender interface {
	Send(ctx context.Context, to, subject, body string) error
}

type sendGridPersonalization struct {
	To []sendGridEmail `json:"to"`
}

type sendGridEmail struct {
	Email string `json:"email"`
}

type sendGridContent struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type sendGridRequest struct {
	Personalizations []sendGridPersonalization `json:"personalizations"`
	From             sendGridEmail             `json:"from"`
	Subject          string                    `json:"subject"`
	Content          []sendGridContent         `json:"content"`
}

// SendGridClient sends emails via the SendGrid API.
type SendGridClient struct {
	apiKey string
	logger *zap.Logger
}

// NewSendGridClient creates a new SendGridClient with the given API key.
func NewSendGridClient(apiKey string, logger *zap.Logger) *SendGridClient {
	return &SendGridClient{apiKey: apiKey, logger: logger}
}

// Send dispatches an email via SendGrid. In this mock, it logs and returns nil.
func (c *SendGridClient) Send(ctx context.Context, to, subject, body string) error {
	reqBody := sendGridRequest{
		Personalizations: []sendGridPersonalization{
			{To: []sendGridEmail{{Email: to}}},
		},
		From:    sendGridEmail{Email: "noreply@pulse-platform.io"},
		Subject: subject,
		Content: []sendGridContent{
			{Type: "text/plain", Value: body},
		},
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal sendgrid request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.sendgrid.com/v3/mail/send", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("build sendgrid request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	c.logger.Info("email sent (mock)", zap.String("to", to), zap.String("subject", subject))
	return nil
}
