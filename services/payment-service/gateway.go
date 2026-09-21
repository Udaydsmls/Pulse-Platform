package main

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Gateway charges a card. It is an interface so the fake below can be swapped
// for a real payment provider later without touching the handler.
type Gateway interface {
	Charge(ctx context.Context, amount float64, currency string) (string, error)
}

// MockGateway is a fake payment provider. It declines anything over
// declineOver, which is how the rollback path can be tried out: order
// something expensive and watch the stock get released.
type MockGateway struct {
	declineOver float64
}

func NewMockGateway(declineOver float64) *MockGateway {
	return &MockGateway{declineOver: declineOver}
}

func (g *MockGateway) Charge(_ context.Context, amount float64, _ string) (string, error) {
	// Pretend to call out to the provider.
	time.Sleep(50 * time.Millisecond)

	if amount > g.declineOver {
		return "", fmt.Errorf("card declined: amount %.2f is over the limit", amount)
	}
	return "txn_" + uuid.NewString(), nil
}
