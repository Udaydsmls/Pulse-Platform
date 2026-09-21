package main

import (
	"context"
	"fmt"
)

// Saga tracks an order across the other services. Each one does its step and
// publishes the result to Kafka; this type reacts and moves the order along.
//
//	order.created -> inventory.reserved -> payment.confirmed -> order.confirmed
//
// If a step fails, order.cancelled is published instead. inventory-service
// listens for it and puts the stock back, which is the rollback.
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

// Cancel marks the order cancelled and publishes order.cancelled, which tells
// inventory-service to release the stock.
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
