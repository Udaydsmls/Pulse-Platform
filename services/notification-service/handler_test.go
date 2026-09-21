package main

import "testing"

func TestDescribeCoversCustomerFacingEvents(t *testing.T) {
	events := []string{
		"user.created",
		"order.created",
		"order.confirmed",
		"order.cancelled",
		"payment.confirmed",
		"payment.failed",
	}

	for _, eventType := range events {
		msg, ok := describe(Event{Type: eventType, OrderID: "order-1"})
		if !ok {
			t.Errorf("describe(%q) returned no message", eventType)
			continue
		}
		if msg.subject == "" || msg.body == "" {
			t.Errorf("describe(%q) has an empty subject or body", eventType)
		}
	}
}

func TestDescribeIgnoresInternalEvents(t *testing.T) {
	// These are bookkeeping between services — nothing to tell the customer.
	for _, eventType := range []string{"inventory.reserved", "inventory.released", "inventory.failed"} {
		if _, ok := describe(Event{Type: eventType}); ok {
			t.Errorf("describe(%q) should not produce a notification", eventType)
		}
	}
}

// Cancellations and payment failures are the ones worth an SMS.
func TestUrgentEventsGoOutBySMS(t *testing.T) {
	for _, eventType := range []string{"order.cancelled", "payment.failed"} {
		msg, _ := describe(Event{Type: eventType})
		if !msg.urgent {
			t.Errorf("describe(%q).urgent = false, want true", eventType)
		}
	}

	msg, _ := describe(Event{Type: "order.confirmed"})
	if msg.urgent {
		t.Error("order.confirmed should not be urgent")
	}
}
