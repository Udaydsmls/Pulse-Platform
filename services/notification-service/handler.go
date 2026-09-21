package main

import (
	"context"
	"fmt"
	"log"
)

// Handler turns events from the other services into customer notifications.
// It listens on every topic, so it is the one place that knows how each event
// should read.
type Handler struct {
	db    *DB
	email Notifier
	sms   Notifier
}

// message is what the customer gets told about an event. Urgent messages also
// go out by SMS.
type message struct {
	subject string
	body    string
	urgent  bool
}

// describe maps an event to its message. Events with no entry here are
// bookkeeping between services and are not worth notifying about.
func describe(event Event) (message, bool) {
	switch event.Type {
	case "user.created":
		return message{
			subject: "Welcome to Pulse",
			body:    "Thanks for signing up.",
		}, true

	case "order.created":
		return message{
			subject: fmt.Sprintf("Order %s received", event.OrderID),
			body:    fmt.Sprintf("We have your order and are checking stock. Total: $%.2f", event.Total),
		}, true

	case "order.confirmed":
		return message{
			subject: fmt.Sprintf("Order %s confirmed", event.OrderID),
			body:    fmt.Sprintf("Your order is confirmed and on its way. Total: $%.2f", event.Total),
		}, true

	case "order.cancelled":
		return message{
			subject: fmt.Sprintf("Order %s cancelled", event.OrderID),
			body:    fmt.Sprintf("Your order was cancelled: %s. You have not been charged.", event.Reason),
			urgent:  true,
		}, true

	case "payment.confirmed":
		return message{
			subject: fmt.Sprintf("Payment received for order %s", event.OrderID),
			body:    fmt.Sprintf("We charged $%.2f. Transaction %s.", event.Total, event.TransactionID),
		}, true

	case "payment.failed":
		return message{
			subject: fmt.Sprintf("Payment problem with order %s", event.OrderID),
			body:    fmt.Sprintf("We could not take payment: %s", event.Reason),
			urgent:  true,
		}, true

	default:
		return message{}, false
	}
}

func (h *Handler) Handle(ctx context.Context, event Event) error {
	msg, ok := describe(event)
	if !ok {
		return nil
	}

	if event.Email == "" {
		return nil
	}

	// Sending is best effort: a provider being down should not stop us
	// recording that the notification was due.
	if err := h.email.Notify(ctx, event.Email, msg.subject, msg.body); err != nil {
		log.Printf("send email to %s: %v", event.Email, err)
	}
	if msg.urgent {
		if err := h.sms.Notify(ctx, event.Email, msg.subject, msg.body); err != nil {
			log.Printf("send sms for %s: %v", event.UserID, err)
		}
	}

	return h.db.Insert(ctx, NewNotification(event.UserID, event.Type, "email", msg.body))
}
