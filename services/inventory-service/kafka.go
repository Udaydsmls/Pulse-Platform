package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

const inventoryEventsTopic = "inventory.events"

// Event is the message shape every Pulse service uses on Kafka. It is one flat
// struct rather than a per-event payload type so producers and consumers can't
// drift apart: a field a service doesn't need is simply empty.
type Event struct {
	Type          string      `json:"event_type"`
	OrderID       string      `json:"order_id,omitempty"`
	UserID        string      `json:"user_id,omitempty"`
	Email         string      `json:"email,omitempty"`
	Items         []OrderItem `json:"items,omitempty"`
	Total         float64     `json:"total,omitempty"`
	TransactionID string      `json:"transaction_id,omitempty"`
	Reason        string      `json:"reason,omitempty"`
	Timestamp     time.Time   `json:"timestamp"`
}

// Producer publishes inventory events.
type Producer struct {
	writer *kafka.Writer
}

func NewProducer(brokers []string) *Producer {
	return &Producer{writer: &kafka.Writer{
		Addr:  kafka.TCP(brokers...),
		Topic: inventoryEventsTopic,
		// Hash the key so every event for one order goes to the same partition
		// and stays in order.
		Balancer:     &kafka.Hash{},
		RequiredAcks: kafka.RequireAll,
	}}
}

func (p *Producer) Publish(ctx context.Context, event Event) error {
	event.Timestamp = time.Now().UTC()

	value, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(event.OrderID),
		Value: value,
	})
}

func (p *Producer) Close() error {
	return p.writer.Close()
}

// Consumer reads events and passes them to handle. Joining a group by ID means
// replicas of this service split the partitions between them, and offsets are
// committed as messages are read.
type Consumer struct {
	reader *kafka.Reader
	handle func(context.Context, Event) error
}

func NewConsumer(brokers, topics []string, groupID string, handle func(context.Context, Event) error) *Consumer {
	return &Consumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:     brokers,
			GroupTopics: topics,
			GroupID:     groupID,
			StartOffset: kafka.FirstOffset,
		}),
		handle: handle,
	}
}

// Run consumes until the context is cancelled.
func (c *Consumer) Run(ctx context.Context) {
	for {
		message, err := c.reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("read message: %v", err)
			continue
		}

		var event Event
		if err := json.Unmarshal(message.Value, &event); err != nil {
			// A message we can't parse will never parse, so skip it rather than
			// retrying it forever.
			log.Printf("skipping malformed message at offset %d: %v", message.Offset, err)
			continue
		}

		if err := c.handle(ctx, event); err != nil {
			log.Printf("handle %s for order %s: %v", event.Type, event.OrderID, err)
		}
	}
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}
