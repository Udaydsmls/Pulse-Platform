package main

import (
	"context"
	"log"
)

// Handler is this service's part of the saga: reserve stock when an order is
// created, and release it again if the order is cancelled.
type Handler struct {
	db       *DB
	producer *Producer
}

func (h *Handler) Handle(ctx context.Context, event Event) error {
	switch event.Type {
	case "order.created":
		return h.reserve(ctx, event)

	// order.cancelled is the saga's compensating event — it is published
	// whenever a later step fails, or when the customer cancels.
	case "order.cancelled":
		return h.release(ctx, event)

	default:
		return nil
	}
}

// reserve tries to hold stock for the order. Both outcomes are published:
// payment-service waits for inventory.reserved, order-service waits for
// inventory.failed.
func (h *Handler) reserve(ctx context.Context, event Event) error {
	if err := h.db.ReserveOrder(ctx, event.OrderID, event.Items); err != nil {
		log.Printf("cannot reserve stock for order %s: %v", event.OrderID, err)
		return h.producer.Publish(ctx, Event{
			Type:    "inventory.failed",
			OrderID: event.OrderID,
			UserID:  event.UserID,
			Email:   event.Email,
			Reason:  err.Error(),
		})
	}

	log.Printf("reserved stock for order %s", event.OrderID)

	// Total and email are copied across so payment-service does not have to
	// ask order-service for them.
	return h.producer.Publish(ctx, Event{
		Type:    "inventory.reserved",
		OrderID: event.OrderID,
		UserID:  event.UserID,
		Email:   event.Email,
		Total:   event.Total,
	})
}

// release puts the order's stock back.
func (h *Handler) release(ctx context.Context, event Event) error {
	if err := h.db.ReleaseOrder(ctx, event.OrderID); err != nil {
		return err
	}

	log.Printf("released stock for order %s", event.OrderID)

	return h.producer.Publish(ctx, Event{
		Type:    "inventory.released",
		OrderID: event.OrderID,
		UserID:  event.UserID,
		Email:   event.Email,
	})
}
