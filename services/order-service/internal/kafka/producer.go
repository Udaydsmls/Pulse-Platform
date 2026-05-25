package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/IBM/sarama"
)

const orderEventsTopic = "order.events"

// EventEnvelope is the standard wrapper for all Kafka domain events.
type EventEnvelope struct {
	EventType   string          `json:"event_type"`
	AggregateID string          `json:"aggregate_id"`
	TenantID    string          `json:"tenant_id"`
	Timestamp   time.Time       `json:"timestamp"`
	Payload     json.RawMessage `json:"payload"`
}

type orderCreatedPayload struct {
	OrderID  string  `json:"order_id"`
	UserID   string  `json:"user_id"`
	TenantID string  `json:"tenant_id"`
	Total    float64 `json:"total"`
}

type orderStatusPayload struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
}

// KafkaProducer wraps a sarama SyncProducer to publish order domain events.
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

// PublishOrderCreated publishes an order.created event to the order.events topic.
func (p *KafkaProducer) PublishOrderCreated(ctx context.Context, orderID, userID, tenantID string, total float64) error {
	return p.publish(orderID, tenantID, "order.created", orderCreatedPayload{
		OrderID:  orderID,
		UserID:   userID,
		TenantID: tenantID,
		Total:    total,
	})
}

// PublishOrderConfirmed publishes an order.confirmed event to the order.events topic.
func (p *KafkaProducer) PublishOrderConfirmed(ctx context.Context, orderID, tenantID string) error {
	return p.publish(orderID, tenantID, "order.confirmed", orderStatusPayload{
		OrderID: orderID,
		Status:  "confirmed",
	})
}

// PublishOrderCancelled publishes an order.cancelled event to the order.events topic.
func (p *KafkaProducer) PublishOrderCancelled(ctx context.Context, orderID, tenantID string) error {
	return p.publish(orderID, tenantID, "order.cancelled", orderStatusPayload{
		OrderID: orderID,
		Status:  "cancelled",
	})
}

func (p *KafkaProducer) publish(aggregateID, tenantID, eventType string, payload interface{}) error {
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal %s payload: %w", eventType, err)
	}

	envelope := EventEnvelope{
		EventType:   eventType,
		AggregateID: aggregateID,
		TenantID:    tenantID,
		Timestamp:   time.Now().UTC(),
		Payload:     json.RawMessage(rawPayload),
	}

	data, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal event envelope: %w", err)
	}

	msg := &sarama.ProducerMessage{
		Topic: orderEventsTopic,
		Key:   sarama.StringEncoder(aggregateID),
		Value: sarama.ByteEncoder(data),
	}

	_, _, err = p.producer.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("send %s event: %w", eventType, err)
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
