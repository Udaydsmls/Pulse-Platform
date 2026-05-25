package domain

import (
	"time"

	"github.com/google/uuid"
)

// PaymentStatus represents the lifecycle state of a payment.
type PaymentStatus string

const (
	StatusPending   PaymentStatus = "pending"
	StatusCompleted PaymentStatus = "completed"
	StatusFailed    PaymentStatus = "failed"
)

// Payment represents a payment transaction for an order.
type Payment struct {
	ID            string
	OrderID       string
	UserID        string
	Amount        float64
	Currency      string
	Status        PaymentStatus
	TransactionID string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// NewPayment creates a new pending Payment for the given order.
func NewPayment(orderID, userID string, amount float64, currency string) *Payment {
	now := time.Now().UTC()
	return &Payment{
		ID:        uuid.NewString(),
		OrderID:   orderID,
		UserID:    userID,
		Amount:    amount,
		Currency:  currency,
		Status:    StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Complete transitions the payment to completed with the given transaction ID.
func (p *Payment) Complete(transactionID string) {
	p.Status = StatusCompleted
	p.TransactionID = transactionID
	p.UpdatedAt = time.Now().UTC()
}

// Fail transitions the payment to failed.
func (p *Payment) Fail(_ string) {
	p.Status = StatusFailed
	p.UpdatedAt = time.Now().UTC()
}
