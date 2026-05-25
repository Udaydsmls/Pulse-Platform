package stripe

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ChargeRequest contains the parameters for a Stripe charge.
type ChargeRequest struct {
	Amount   float64
	Currency string
	Token    string
	OrderID  string
}

// ChargeResult holds the outcome of a charge attempt.
type ChargeResult struct {
	TransactionID string
	Success       bool
}

// StripeClient defines the interface for processing charges.
type StripeClient interface {
	Charge(ctx context.Context, req *ChargeRequest) (*ChargeResult, error)
}

// MockStripeClient is a test-double implementation of StripeClient.
type MockStripeClient struct{}

// NewMockStripeClient creates a new MockStripeClient.
func NewMockStripeClient() *MockStripeClient {
	return &MockStripeClient{}
}

// Charge simulates a payment charge. Fails when the token is "fail_token".
func (m *MockStripeClient) Charge(_ context.Context, req *ChargeRequest) (*ChargeResult, error) {
	time.Sleep(50 * time.Millisecond)
	if req.Token == "fail_token" {
		return &ChargeResult{Success: false}, nil
	}
	return &ChargeResult{
		TransactionID: uuid.NewString(),
		Success:       true,
	}, nil
}
