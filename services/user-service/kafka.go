package main

import (
	"context"
	"encoding/json"
	"time"

	"github.com/segmentio/kafka-go"
)

const userEventsTopic = "user.events"

// Event is the JSON message shape shared by all the services. This one only
// publishes new accounts, so it needs just these fields.
type Event struct {
	Type      string    `json:"eventType"`
	UserID    string    `json:"userId,omitempty"`
	Email     string    `json:"email,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// Producer publishes user events to Kafka.
type Producer struct {
	writer *kafka.Writer
}

func NewProducer(brokers []string) *Producer {
	return &Producer{writer: &kafka.Writer{
		Addr:     kafka.TCP(brokers...),
		Topic:    userEventsTopic,
		Balancer: &kafka.Hash{},
		// Wait for the brokers to confirm the write before returning.
		RequiredAcks: kafka.RequireAll,
	}}
}

// PublishUserCreated announces a new account so notification-service can send
// the welcome email.
func (p *Producer) PublishUserCreated(ctx context.Context, userID, email string) error {
	value, err := json.Marshal(Event{
		Type:      "user.created",
		UserID:    userID,
		Email:     email,
		Timestamp: time.Now().UTC(),
	})
	if err != nil {
		return err
	}

	return p.writer.WriteMessages(ctx, kafka.Message{Key: []byte(userID), Value: value})
}

func (p *Producer) Close() error {
	return p.writer.Close()
}
