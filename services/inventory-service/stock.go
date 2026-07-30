package main

import "time"

// StockItem is the stock we hold for one product. StockLevel is the physical
// count; Reserved is how much of it is already promised to pending orders.
type StockItem struct {
	ProductID  string
	StockLevel int32
	Reserved   int32
	UpdatedAt  time.Time
}

// Available is how much can still be reserved.
func (s *StockItem) Available() int32 {
	return s.StockLevel - s.Reserved
}

// OrderItem is one line of an order, as it arrives on order.created.
type OrderItem struct {
	ProductID string  `json:"product_id"`
	Quantity  int32   `json:"quantity"`
	UnitPrice float64 `json:"unit_price"`
}
