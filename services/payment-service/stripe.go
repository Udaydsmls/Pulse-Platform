package main

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Gateway is the card processor. Keeping it an interface means the mock below
// can be swapped for the real Stripe SDK without touching the saga handler.
type Gateway interface {
	Charge(ctx context.Context, amount float64, currency string) (string, error)
}

// MockGateway stands in for Stripe. It declines anything over declineOver so the
// saga's rollback path can be exercised end to end: order something expensive
// and watch the stock get released.
//
// The saga rolls back on any charge failure, so a decline and an outage are
// handled the same way and don't need to be told apart.
type MockGateway struct {
	declineOver float64
}

func NewMockGateway(declineOver float64) *MockGateway {
	return &MockGateway{declineOver: declineOver}
}

func (g *MockGateway) Charge(_ context.Context, amount float64, _ string) (string, error) {
	// Stand in for the network round trip to the processor.
	time.Sleep(50 * time.Millisecond)

	if amount > g.declineOver {
		return "", fmt.Errorf("card declined: amount %.2f is over the limit", amount)
	}
	return "txn_" + uuid.NewString(), nil
}
