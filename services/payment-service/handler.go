package main

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
)

const defaultCurrency = "USD"

// Handler is this service's step in the saga: charge the customer once the
// stock has been reserved.
type Handler struct {
	db       *DB
	gateway  Gateway
	producer *Producer
}

func (h *Handler) Handle(ctx context.Context, event Event) error {
	if event.Type != "inventory.reserved" {
		return nil
	}
	return h.charge(ctx, event)
}

// charge saves a pending payment, calls the gateway, then publishes the
// result. order-service is waiting for payment.confirmed to finish the order,
// or payment.failed to roll it back.
func (h *Handler) charge(ctx context.Context, event Event) error {
	payment := &Payment{
		ID:        uuid.NewString(),
		OrderID:   event.OrderID,
		UserID:    event.UserID,
		Amount:    event.Total,
		Currency:  defaultCurrency,
		Status:    StatusPending,
		CreatedAt: time.Now().UTC(),
	}

	// Without a payment row there is no record of the charge, so stop here
	// rather than charging the card anyway.
	if err := h.db.Insert(ctx, payment); err != nil {
		return err
	}

	transactionID, err := h.gateway.Charge(ctx, payment.Amount, payment.Currency)
	if err != nil {
		return h.fail(ctx, payment, event, err.Error())
	}

	payment.Status = StatusCompleted
	payment.TransactionID = transactionID
	if err := h.db.UpdateStatus(ctx, payment.ID, payment.Status, payment.TransactionID); err != nil {
		log.Printf("record completed payment %s: %v", payment.ID, err)
	}

	log.Printf("charged %.2f %s for order %s", payment.Amount, payment.Currency, payment.OrderID)

	return h.producer.Publish(ctx, Event{
		Type:          "payment.confirmed",
		OrderID:       event.OrderID,
		UserID:        event.UserID,
		Email:         event.Email,
		Total:         payment.Amount,
		TransactionID: transactionID,
	})
}

// fail marks the payment failed and publishes payment.failed, which starts the
// rollback.
func (h *Handler) fail(ctx context.Context, payment *Payment, event Event, reason string) error {
	payment.Status = StatusFailed
	if err := h.db.UpdateStatus(ctx, payment.ID, payment.Status, ""); err != nil {
		log.Printf("record failed payment %s: %v", payment.ID, err)
	}

	log.Printf("payment failed for order %s: %s", payment.OrderID, reason)

	return h.producer.Publish(ctx, Event{
		Type:    "payment.failed",
		OrderID: event.OrderID,
		UserID:  event.UserID,
		Email:   event.Email,
		Reason:  reason,
	})
}
