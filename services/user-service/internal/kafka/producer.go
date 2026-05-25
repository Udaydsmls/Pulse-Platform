package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/IBM/sarama"
)

const userEventsTopic = "user.events"

// EventEnvelope is the standard wrapper for all Kafka domain events.
type EventEnvelope struct {
	EventType   string          `json:"event_type"`
	AggregateID string          `json:"aggregate_id"`
	TenantID    string          `json:"tenant_id"`
	Timestamp   time.Time       `json:"timestamp"`
	Payload     json.RawMessage `json:"payload"`
}

type userCreatedPayload struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
}

// KafkaProducer wraps a sarama SyncProducer to publish user domain events.
type KafkaProducer struct {
	producer sarama.SyncProducer
}

// NewProducer creates a KafkaProducer connected to the given brokers.
func NewProducer(brokers []string) (*KafkaProducer, error) {
	cfg := sarama.NewConfig()
	cfg.Producer.Return.Successes = true
	cfg.Producer.RequiredAcks = sarama.WaitForAll
	cfg.Producer.Retry.Max = 3

	producer, err := sarama.NewSyncProducer(brokers, cfg)
	if err != nil {
		return nil, fmt.Errorf("create kafka producer: %w", err)
	}
	return &KafkaProducer{producer: producer}, nil
}

// PublishUserCreated publishes a user.created event to the user.events topic.
func (p *KafkaProducer) PublishUserCreated(ctx context.Context, userID, email, tenantID string) error {
	payload, err := json.Marshal(userCreatedPayload{UserID: userID, Email: email})
	if err != nil {
		return fmt.Errorf("marshal user created payload: %w", err)
	}

	envelope := EventEnvelope{
		EventType:   "user.created",
		AggregateID: userID,
		TenantID:    tenantID,
		Timestamp:   time.Now().UTC(),
		Payload:     json.RawMessage(payload),
	}

	data, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal event envelope: %w", err)
	}

	msg := &sarama.ProducerMessage{
		Topic: userEventsTopic,
		Key:   sarama.StringEncoder(userID),
		Value: sarama.ByteEncoder(data),
	}

	_, _, err = p.producer.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("send user.created event: %w", err)
	}
	return nil
}

// Close shuts down the underlying Kafka producer.
func (p *KafkaProducer) Close() error {
	if err := p.producer.Close(); err != nil {
		return fmt.Errorf("close kafka producer: %w", err)
	}
	return nil
}
