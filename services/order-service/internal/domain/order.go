package domain

import (
	"errors"
	"fmt"
	"time"
)

// OrderStatus represents the lifecycle state of an order.
type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "pending"
	OrderStatusConfirmed OrderStatus = "confirmed"
	OrderStatusCancelled OrderStatus = "cancelled"
	OrderStatusFailed    OrderStatus = "failed"
)

// OrderItem represents a single line item within an order.
type OrderItem struct {
	ProductID string
	Quantity  int
	UnitPrice float64
}

// Order is the core domain entity representing a customer order.
type Order struct {
	ID        string
	UserID    string
	TenantID  string
	Status    OrderStatus
	Items     []OrderItem
	Total     float64
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewOrder creates a new Order in pending status with a calculated total.
func NewOrder(userID, tenantID string, items []OrderItem) *Order {
	now := time.Now().UTC()
	o := &Order{
		UserID:    userID,
		TenantID:  tenantID,
		Status:    OrderStatusPending,
		Items:     items,
		CreatedAt: now,
		UpdatedAt: now,
	}
	o.Total = o.CalculateTotal()
	return o
}

// CalculateTotal sums the extended price of all order items.
func (o *Order) CalculateTotal() float64 {
	var total float64
	for _, item := range o.Items {
		total += float64(item.Quantity) * item.UnitPrice
	}
	return total
}

// Confirm transitions the order to confirmed status.
func (o *Order) Confirm() {
	o.Status = OrderStatusConfirmed
	o.UpdatedAt = time.Now().UTC()
}

// Cancel transitions the order to cancelled status.
func (o *Order) Cancel() {
	o.Status = OrderStatusCancelled
	o.UpdatedAt = time.Now().UTC()
}

// Fail transitions the order to failed status.
func (o *Order) Fail() {
	o.Status = OrderStatusFailed
	o.UpdatedAt = time.Now().UTC()
}

// Validate checks that the Order has all required fields and at least one item.
func (o *Order) Validate() error {
	if o.UserID == "" {
		return errors.New("user_id is required")
	}
	if o.TenantID == "" {
		return errors.New("tenant_id is required")
	}
	if len(o.Items) == 0 {
		return errors.New("order must contain at least one item")
	}
	for i, item := range o.Items {
		if item.ProductID == "" {
			return fmt.Errorf("item[%d]: product_id is required", i)
		}
		if item.Quantity <= 0 {
			return fmt.Errorf("item[%d]: quantity must be positive", i)
		}
		if item.UnitPrice < 0 {
			return fmt.Errorf("item[%d]: unit_price must be non-negative", i)
		}
	}
	return nil
}
