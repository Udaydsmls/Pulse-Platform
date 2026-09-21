package main

import (
	"errors"
	"fmt"
	"time"
)

// Order statuses. An order starts pending and ends up confirmed or cancelled.
const (
	StatusPending   = "pending"
	StatusConfirmed = "confirmed"
	StatusCancelled = "cancelled"
)

// OrderItem is one line in an order.
type OrderItem struct {
	ProductID string  `json:"productId"`
	Quantity  int32   `json:"quantity"`
	UnitPrice float64 `json:"unitPrice"`
}

// Order is a customer order.
type Order struct {
	ID        string
	UserID    string
	Email     string
	Status    string
	Items     []OrderItem
	Total     float64
	CreatedAt time.Time
}

// NewOrder builds a pending order and adds up the total.
func NewOrder(id, userID, email string, items []OrderItem) *Order {
	order := &Order{
		ID:        id,
		UserID:    userID,
		Email:     email,
		Status:    StatusPending,
		Items:     items,
		CreatedAt: time.Now().UTC(),
	}

	for _, item := range items {
		order.Total += float64(item.Quantity) * item.UnitPrice
	}
	return order
}

func (o *Order) Validate() error {
	if o.UserID == "" {
		return errors.New("user_id is required")
	}
	if len(o.Items) == 0 {
		return errors.New("order must have at least one item")
	}

	for i, item := range o.Items {
		if item.ProductID == "" {
			return fmt.Errorf("item %d: product_id is required", i)
		}
		if item.Quantity <= 0 {
			return fmt.Errorf("item %d: quantity must be positive", i)
		}
		if item.UnitPrice < 0 {
			return fmt.Errorf("item %d: unit_price cannot be negative", i)
		}
	}
	return nil
}

// CanCancel reports whether the order can still be cancelled.
func (o *Order) CanCancel() bool {
	return o.Status == StatusPending || o.Status == StatusConfirmed
}
