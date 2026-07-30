package main

import "testing"

func TestDescribeCoversSagaEvents(t *testing.T) {
	// Every event the customer should hear about, and the status the browser
	// shows for it.
	want := map[string]string{
		"user.created":      "registered",
		"order.created":     "pending",
		"order.confirmed":   "confirmed",
		"order.cancelled":   "cancelled",
		"payment.confirmed": "paid",
		"payment.failed":    "payment_failed",
	}

	for eventType, wantStatus := range want {
		msg, ok := describe(Event{Type: eventType, OrderID: "order-1"})
		if !ok {
			t.Errorf("describe(%q) returned no message", eventType)
			continue
		}
		if msg.status != wantStatus {
			t.Errorf("describe(%q).status = %q, want %q", eventType, msg.status, wantStatus)
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
