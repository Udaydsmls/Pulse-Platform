package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// StockItem represents the inventory stock for a product.
type StockItem struct {
	ID         string
	ProductID  string
	StockLevel int32
	Reserved   int32
	UpdatedAt  time.Time
}

// Reservation represents a stock reservation for an order.
type Reservation struct {
	ID        string
	OrderID   string
	ProductID string
	Quantity  int32
	Status    string
	CreatedAt time.Time
}

// NewStockItem creates a new StockItem for the given product with an initial stock level.
func NewStockItem(productID string, initialLevel int32) *StockItem {
	return &StockItem{
		ID:         uuid.NewString(),
		ProductID:  productID,
		StockLevel: initialLevel,
		Reserved:   0,
		UpdatedAt:  time.Now().UTC(),
	}
}

// NewReservation creates a new active Reservation for the given order and product.
func NewReservation(orderID, productID string, quantity int32) *Reservation {
	return &Reservation{
		ID:        uuid.NewString(),
		OrderID:   orderID,
		ProductID: productID,
		Quantity:  quantity,
		Status:    "active",
		CreatedAt: time.Now().UTC(),
	}
}

// Available returns the number of units available for reservation.
func (s *StockItem) Available() int32 {
	return s.StockLevel - s.Reserved
}

// Reserve decrements the available stock by the requested quantity.
// Returns an error if there is insufficient stock.
func (s *StockItem) Reserve(quantity int32) error {
	if s.Available() < quantity {
		return errors.New("insufficient stock")
	}
	s.Reserved += quantity
	s.UpdatedAt = time.Now().UTC()
	return nil
}

// Release increments available stock by returning the reserved quantity.
func (s *StockItem) Release(quantity int32) {
	if s.Reserved >= quantity {
		s.Reserved -= quantity
	} else {
		s.Reserved = 0
	}
	s.UpdatedAt = time.Now().UTC()
}
