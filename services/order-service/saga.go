package main

import (
	"context"
	"fmt"
)

// Saga drives the distributed order workflow. Each service does one step and
// announces the result on Kafka; this type reacts to those results and moves
// the order forward.
//
// Happy path:
//
//	CreateOrder -> order.created
//	           -> inventory-service reserves stock -> inventory.reserved
//	           -> payment-service charges the card -> payment.confirmed
//	           -> order.confirmed
//
// There is no distributed transaction to roll back, so every failure step
// publishes order.cancelled as its compensating event. inventory-service
// listens for it and releases whatever it reserved:
//
//	inventory.failed -> order.cancelled   (nothing reserved yet)
//	payment.failed   -> order.cancelled   -> stock released
//	CancelOrder RPC  -> order.cancelled   -> stock released
//
// Releasing is keyed by order ID and only touches reservations still marked
// active, so an order.cancelled with nothing to release is a no-op. That means
// duplicate deliveries are safe.
type Saga struct {
	db       *DB
	producer *Producer
}

// Handle reacts to an inventory or payment event.
func (s *Saga) Handle(ctx context.Context, event Event) error {
	switch event.Type {
	case "payment.confirmed":
		return s.Confirm(ctx, event.OrderID, event.TransactionID)

	case "inventory.failed", "payment.failed":
		return s.Cancel(ctx, event.OrderID, event.Reason)

	default:
		// Other services' events (inventory.reserved, inventory.released) are
		// not ours to act on.
		return nil
	}
}

// Confirm marks the order confirmed and publishes order.confirmed.
func (s *Saga) Confirm(ctx context.Context, orderID, transactionID string) error {
	order, err := s.db.FindByID(ctx, orderID)
	if err != nil {
		return fmt.Errorf("find order %s: %w", orderID, err)
	}

	if err := s.db.UpdateStatus(ctx, orderID, StatusConfirmed); err != nil {
		return fmt.Errorf("confirm order %s: %w", orderID, err)
	}

	return s.producer.Publish(ctx, Event{
		Type:          "order.confirmed",
		OrderID:       order.ID,
		UserID:        order.UserID,
		Email:         order.Email,
		Total:         order.Total,
		TransactionID: transactionID,
	})
}

// Cancel marks the order cancelled and publishes order.cancelled, which is the
// compensating event that tells inventory-service to release stock.
func (s *Saga) Cancel(ctx context.Context, orderID, reason string) error {
	order, err := s.db.FindByID(ctx, orderID)
	if err != nil {
		return fmt.Errorf("find order %s: %w", orderID, err)
	}

	if err := s.db.UpdateStatus(ctx, orderID, StatusCancelled); err != nil {
		return fmt.Errorf("cancel order %s: %w", orderID, err)
	}

	return s.producer.Publish(ctx, Event{
		Type:    "order.cancelled",
		OrderID: order.ID,
		UserID:  order.UserID,
		Email:   order.Email,
		Reason:  reason,
	})
}
