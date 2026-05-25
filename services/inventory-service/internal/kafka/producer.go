package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/IBM/sarama"
)

// KafkaProducer publishes inventory domain events to Kafka.
type KafkaProducer struct {
	producer sarama.SyncProducer
}

// NewKafkaProducer creates a new KafkaProducer connected to the given brokers.
func NewKafkaProducer(brokers []string) (*KafkaProducer, error) {
	cfg := sarama.NewConfig()
	cfg.Producer.Return.Successes = true
	cfg.Producer.RequiredAcks = sarama.WaitForAll

	producer, err := sarama.NewSyncProducer(brokers, cfg)
	if err != nil {
		return nil, fmt.Errorf("create kafka producer: %w", err)
	}
	return &KafkaProducer{producer: producer}, nil
}

// PublishInventoryReserved publishes an inventory.reserved event.
func (p *KafkaProducer) PublishInventoryReserved(ctx context.Context, orderID, reservationID, tenantID string) error {
	payload, err := json.Marshal(map[string]string{
		"order_id":       orderID,
		"reservation_id": reservationID,
	})
	if err != nil {
		return fmt.Errorf("marshal inventory reserved payload: %w", err)
	}
	return p.publish(orderID, tenantID, "inventory.reserved", payload)
}

// PublishInventoryFailed publishes an inventory.failed event.
func (p *KafkaProducer) PublishInventoryFailed(ctx context.Context, orderID, reason, tenantID string) error {
	payload, err := json.Marshal(map[string]string{
		"order_id": orderID,
		"reason":   reason,
	})
	if err != nil {
		return fmt.Errorf("marshal inventory failed payload: %w", err)
	}
	return p.publish(orderID, tenantID, "inventory.failed", payload)
}

// Close shuts down the producer cleanly.
func (p *KafkaProducer) Close() error {
	return p.producer.Close()
}

func (p *KafkaProducer) publish(aggregateID, tenantID, eventType string, payload []byte) error {
	envelope := EventEnvelope{
		EventType:   eventType,
		AggregateID: aggregateID,
		TenantID:    tenantID,
		Timestamp:   time.Now().UTC(),
		Payload:     payload,
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal event envelope: %w", err)
	}
	msg := &sarama.ProducerMessage{
		Topic: "inventory.events",
		Key:   sarama.StringEncoder(aggregateID),
		Value: sarama.ByteEncoder(data),
	}
	_, _, err = p.producer.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("send kafka message: %w", err)
	}
	return nil
}
