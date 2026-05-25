package notification

import (
	"context"

	"go.uber.org/zap"
)

// SMSSender defines the interface for sending SMS notifications.
type SMSSender interface {
	Send(ctx context.Context, to, body string) error
}

// TwilioClient sends SMS messages via the Twilio API.
type TwilioClient struct {
	accountSID string
	authToken  string
	logger     *zap.Logger
}

// NewTwilioClient creates a new TwilioClient with the given credentials.
func NewTwilioClient(accountSID, authToken string, logger *zap.Logger) *TwilioClient {
	return &TwilioClient{accountSID: accountSID, authToken: authToken, logger: logger}
}

// Send dispatches an SMS via Twilio. In this mock, it logs and returns nil.
func (c *TwilioClient) Send(_ context.Context, to, body string) error {
	c.logger.Info("SMS sent (mock)", zap.String("to", to), zap.String("body", body))
	return nil
}
