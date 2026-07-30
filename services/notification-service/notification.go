package main

import (
	"context"
	"log"
	"time"
)

// Log is one dispatched notification, stored in DynamoDB so there is a record
// of what each customer was told and when.
type Log struct {
	UserID    string
	Timestamp string
	EventType string
	Channel   string
	Message   string
}

// NewLog builds a log entry. Timestamp is the DynamoDB sort key, so it is
// stored as an RFC 3339 string that sorts chronologically.
func NewLog(userID, eventType, channel, message string) *Log {
	return &Log{
		UserID:    userID,
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		EventType: eventType,
		Channel:   channel,
		Message:   message,
	}
}

// Notifier delivers a message to a customer. Swapping in SendGrid or Twilio
// means implementing this interface.
type Notifier interface {
	Notify(ctx context.Context, to, subject, body string) error
}

// LogNotifier is a stand-in for a real email/SMS provider: it writes what it
// would have sent to the service log. Keeps the project runnable without
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
