package main

import (
	"context"
	"encoding/json"
	"time"

	"github.com/segmentio/kafka-go"
)

const userEventsTopic = "user.events"

// Event is the message shape every Pulse service uses on Kafka. This service
// only announces new accounts, so it needs just these fields — the other
// services' events carry more.
type Event struct {
	Type      string    `json:"event_type"`
	UserID    string    `json:"user_id,omitempty"`
	Email     string    `json:"email,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// Producer publishes user events.
type Producer struct {
	writer *kafka.Writer
}

func NewProducer(brokers []string) *Producer {
	return &Producer{writer: &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        userEventsTopic,
		Balancer:     &kafka.Hash{},
		RequiredAcks: kafka.RequireAll,
	}}
}

// PublishUserCreated announces a new account so notification-service can send
// the welcome email.
func (p *Producer) PublishUserCreated(ctx context.Context, userID, email string) error {
	event := Event{
		Type:      "user.created",
		UserID:    userID,
		Email:     email,
		Timestamp: time.Now().UTC(),
	}

	value, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(userID),
		Value: value,
	})
}

func (p *Producer) Close() error {
	return p.writer.Close()
}
