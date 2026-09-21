package main

import "testing"

// Available has to exclude stock already promised to other pending orders.
func TestAvailableExcludesReservedStock(t *testing.T) {
	tests := []struct {
		name       string
		stockLevel int32
		reserved   int32
		want       int32
	}{
		{name: "nothing reserved", stockLevel: 10, reserved: 0, want: 10},
		{name: "some reserved", stockLevel: 10, reserved: 4, want: 6},
		{name: "all reserved", stockLevel: 10, reserved: 10, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := &StockItem{StockLevel: tt.stockLevel, Reserved: tt.reserved}
			if got := item.Available(); got != tt.want {
				t.Errorf("Available() = %d, want %d", got, tt.want)
			}
		})
	}
}
