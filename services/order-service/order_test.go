package main

import "testing"

func TestNewOrderCalculatesTotal(t *testing.T) {
	order := NewOrder("order-1", "user-1", "user@example.com", []OrderItem{
		{ProductID: "p1", Quantity: 2, UnitPrice: 10.50},
		{ProductID: "p2", Quantity: 1, UnitPrice: 4.00},
	})

	if order.Total != 25.00 {
		t.Errorf("Total = %v, want 25.00", order.Total)
	}
	if order.Status != StatusPending {
		t.Errorf("Status = %q, want %q", order.Status, StatusPending)
	}
}

func TestOrderValidate(t *testing.T) {
	validItems := []OrderItem{{ProductID: "p1", Quantity: 1, UnitPrice: 9.99}}

	tests := []struct {
		name    string
		order   *Order
		wantErr bool
	}{
		{
			name:  "valid order",
			order: NewOrder("order-1", "user-1", "user@example.com", validItems),
		},
		{
			name:    "no user",
			order:   NewOrder("order-1", "", "user@example.com", validItems),
			wantErr: true,
		},
		{
			name:    "no items",
			order:   NewOrder("order-1", "user-1", "user@example.com", nil),
			wantErr: true,
		},
		{
			name: "zero quantity",
			order: NewOrder("order-1", "user-1", "user@example.com", []OrderItem{
				{ProductID: "p1", Quantity: 0, UnitPrice: 9.99},
			}),
			wantErr: true,
		},
		{
			name: "missing product id",
			order: NewOrder("order-1", "user-1", "user@example.com", []OrderItem{
				{ProductID: "", Quantity: 1, UnitPrice: 9.99},
			}),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.order.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// A cancelled order must not be cancellable again, or the saga would publish a
// second order.cancelled and release stock twice.
func TestCanCancel(t *testing.T) {
	tests := map[string]bool{
		StatusPending:   true,
		StatusConfirmed: true,
		StatusCancelled: false,
	}

	for status, want := range tests {
		order := &Order{Status: status}
		if got := order.CanCancel(); got != want {
			t.Errorf("status %q: CanCancel() = %v, want %v", status, got, want)
		}
	}
}
