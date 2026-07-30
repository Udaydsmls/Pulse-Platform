package main

import (
	"context"
	"fmt"
	"log"
)

// Handler turns domain events into customer-facing notifications. It listens on
// every topic, so it is the one place that knows how each event should read.
//
// It also republishes each notification to notification.events, which is the
// first hop of the real-time pipeline:
//
//	notification.events -> api-gateway consumer -> Redis pub/sub -> WebSocket
type Handler struct {
	store    *Store
	email    Notifier
	sms      Notifier
	producer *Producer
}

// message is what we tell the customer about an event.
type message struct {
	subject string
	body    string
	// status is the short machine-readable label the browser UI displays.
	status string
	// urgent messages also go out by SMS.
	urgent bool
}

// describe maps an event to its customer-facing message. Events with no entry
// here are internal bookkeeping and are not worth notifying about.
func describe(event Event) (message, bool) {
	switch event.Type {
	case "user.created":
		return message{
			subject: "Welcome to Pulse",
			body:    "Thanks for signing up.",
			status:  "registered",
		}, true

	case "order.created":
		return message{
			subject: fmt.Sprintf("Order %s received", event.OrderID),
			body:    fmt.Sprintf("We have your order and are checking stock. Total: $%.2f", event.Total),
			status:  "pending",
		}, true

	case "order.confirmed":
		return message{
			subject: fmt.Sprintf("Order %s confirmed", event.OrderID),
			body:    fmt.Sprintf("Your order is confirmed and on its way. Total: $%.2f", event.Total),
			status:  "confirmed",
		}, true

	case "order.cancelled":
		return message{
			subject: fmt.Sprintf("Order %s cancelled", event.OrderID),
			body:    fmt.Sprintf("Your order was cancelled: %s. You have not been charged.", event.Reason),
			status:  "cancelled",
			urgent:  true,
		}, true

	case "payment.confirmed":
		return message{
			subject: fmt.Sprintf("Payment received for order %s", event.OrderID),
			body:    fmt.Sprintf("We charged $%.2f. Transaction %s.", event.Total, event.TransactionID),
			status:  "paid",
		}, true

	case "payment.failed":
		return message{
			subject: fmt.Sprintf("Payment problem with order %s", event.OrderID),
			body:    fmt.Sprintf("We could not take payment: %s", event.Reason),
			status:  "payment_failed",
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

	// Delivery is best effort: a provider being down should not stop us
	// recording the notification or pushing the update to the customer's open
	// browser tab.
	if event.Email != "" {
		if err := h.email.Notify(ctx, event.Email, msg.subject, msg.body); err != nil {
			log.Printf("send email to %s: %v", event.Email, err)
		}
	}
	if msg.urgent && event.Email != "" {
		if err := h.sms.Notify(ctx, event.Email, msg.subject, msg.body); err != nil {
			log.Printf("send sms for %s: %v", event.UserID, err)
		}
	}

	if err := h.store.Save(ctx, NewLog(event.UserID, event.Type, "email", msg.body)); err != nil {
		log.Printf("save notification log for %s: %v", event.UserID, err)
	}

	// This is what reaches the browser over the WebSocket.
	return h.producer.Publish(ctx, Event{
		Type:    event.Type,
		OrderID: event.OrderID,
		UserID:  event.UserID,
		Status:  msg.status,
		Message: msg.body,
	})
}
