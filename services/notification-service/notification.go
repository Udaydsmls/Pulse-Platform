package main

import (
	"context"
	"log"
	"time"
)

// Notification is one message that was sent to a customer.
type Notification struct {
	UserID    string
	EventType string
	Channel   string
	Message   string
	CreatedAt time.Time
}

func NewNotification(userID, eventType, channel, message string) *Notification {
	return &Notification{
		UserID:    userID,
		EventType: eventType,
		Channel:   channel,
		Message:   message,
		CreatedAt: time.Now().UTC(),
	}
}

// Notifier delivers a message to a customer.
type Notifier interface {
	Notify(ctx context.Context, to, subject, body string) error
}

// LogNotifier stands in for a real email or SMS provider by writing what it
// would have sent to the log. It keeps the project runnable without any
// third-party API keys.
type LogNotifier struct {
	channel string
}

func NewLogNotifier(channel string) *LogNotifier {
	return &LogNotifier{channel: channel}
}

func (n *LogNotifier) Notify(_ context.Context, to, subject, body string) error {
	log.Printf("[%s] to=%s subject=%q body=%q", n.channel, to, subject, body)
	return nil
}
