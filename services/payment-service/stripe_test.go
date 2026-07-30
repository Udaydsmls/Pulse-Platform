package main

import (
	"context"
	"testing"
)

func TestMockGatewayApprovesNormalAmounts(t *testing.T) {
	gateway := NewMockGateway(5000)

	transactionID, err := gateway.Charge(context.Background(), 99.99, "USD")
	if err != nil {
		t.Fatalf("Charge() error = %v, want nil", err)
	}
	if transactionID == "" {
		t.Error("Charge() returned an empty transaction ID")
	}
}

// The saga's rollback path depends on this decline, so it has to actually fail.
func TestMockGatewayDeclinesOverLimit(t *testing.T) {
	gateway := NewMockGateway(5000)

	if _, err := gateway.Charge(context.Background(), 5000.01, "USD"); err == nil {
		t.Error("Charge() error = nil, want a decline")
	}
}
