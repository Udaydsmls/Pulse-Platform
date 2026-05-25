package domain

import (
	"time"
)

// NotificationLog records a notification that was dispatched for a domain event.
type NotificationLog struct {
	UserID    string
	Timestamp string
	EventType string
	Channel   string
	Status    string
	Payload   string
	CreatedAt time.Time
}

// NewNotificationLog creates a new NotificationLog for the given user and event.
// Timestamp is set to an ISO 8601 string and is used as the DynamoDB sort key.
func NewNotificationLog(userID, eventType, channel, payload string) *NotificationLog {
	now := time.Now().UTC()
	return &NotificationLog{
		UserID:    userID,
		Timestamp: now.Format(time.RFC3339Nano),
		EventType: eventType,
		Channel:   channel,
		Status:    "sent",
		Payload:   payload,
		CreatedAt: now,
	}
}
