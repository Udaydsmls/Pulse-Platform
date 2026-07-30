package main

import "time"

// Payment statuses.
const (
	StatusPending   = "pending"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
)

// Payment is one charge attempt against an order.
type Payment struct {
	ID            string
	OrderID       string
	UserID        string
	Amount        float64
	Currency      string
	Status        string
	TransactionID string
	CreatedAt     time.Time
}
