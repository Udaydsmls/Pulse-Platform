package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

const paymentEventsTopic = "payment.events"

// Event is the JSON message shape shared by all the services. Fields a service
// does not need are just left empty.
type Event struct {
	Type          string    `json:"eventType"`
	OrderID       string    `json:"orderId,omitempty"`
	UserID        string    `json:"userId,omitempty"`
	Email         string    `json:"email,omitempty"`
	Total         float64   `json:"total,omitempty"`
	TransactionID string    `json:"transactionId,omitempty"`
	Reason        string    `json:"reason,omitempty"`
	Timestamp     time.Time `json:"timestamp"`
}

// Producer publishes payment events to Kafka.
type Producer struct {
	writer *kafka.Writer
}

func NewProducer(brokers []string) *Producer {
	return &Producer{writer: &kafka.Writer{
		Addr:  kafka.TCP(brokers...),
		Topic: paymentEventsTopic,
		// Keying by order ID keeps all the events for one order in order.
		Balancer: &kafka.Hash{},
		// Wait for the brokers to confirm the write before returning.
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

// Consumer reads events from Kafka and passes them to handle.
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

// Run consumes messages until the context is cancelled.
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
			log.Printf("skipping bad message at offset %d: %v", message.Offset, err)
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
