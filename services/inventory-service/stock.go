package main

import "time"

// StockItem is the stock held for one product. StockLevel is how many units
// exist; Reserved is how many are already promised to pending orders.
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
	ProductID string  `json:"productId"`
	Quantity  int32   `json:"quantity"`
	UnitPrice float64 `json:"unitPrice"`
}
