package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"go.uber.org/zap"
)

// EventEnvelope is the standard wrapper for all domain events published to Kafka.
type EventEnvelope struct {
	EventType   string          `json:"event_type"`
	AggregateID string          `json:"aggregate_id"`
	TenantID    string          `json:"tenant_id"`
	Timestamp   time.Time       `json:"timestamp"`
	Payload     json.RawMessage `json:"payload"`
}

// EventHandler processes a received EventEnvelope.
type EventHandler interface {
	Handle(ctx context.Context, envelope EventEnvelope) error
}

// KafkaConsumer wraps a sarama consumer group to consume order events.
type KafkaConsumer struct {
	group   sarama.ConsumerGroup
	topics  []string
	handler EventHandler
	logger  *zap.Logger
}

// NewConsumer creates a new KafkaConsumer that consumes the "order.events" topic.
func NewConsumer(brokers []string, groupID string, handler EventHandler, logger *zap.Logger) (*KafkaConsumer, error) {
	cfg := sarama.NewConfig()
	cfg.Consumer.Offsets.Initial = sarama.OffsetNewest
	cfg.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRoundRobin()}

	group, err := sarama.NewConsumerGroup(brokers, groupID, cfg)
	if err != nil {
		return nil, fmt.Errorf("create consumer group: %w", err)
	}
	return &KafkaConsumer{
		group:   group,
		topics:  []string{"order.events"},
		handler: handler,
		logger:  logger,
	}, nil
}

// Start begins consuming messages and blocks until the context is cancelled.
func (c *KafkaConsumer) Start(ctx context.Context) error {
	h := &consumerGroupHandler{handler: c.handler, logger: c.logger}
	for {
		if err := c.group.Consume(ctx, c.topics, h); err != nil {
			return fmt.Errorf("consumer group error: %w", err)
		}
		if ctx.Err() != nil {
			return nil
		}
	}
}

// Close shuts down the consumer group.
func (c *KafkaConsumer) Close() error {
	return c.group.Close()
}

type consumerGroupHandler struct {
	handler EventHandler
	logger  *zap.Logger
}

func (h *consumerGroupHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *consumerGroupHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		var envelope EventEnvelope
		if err := json.Unmarshal(msg.Value, &envelope); err != nil {
			h.logger.Error("failed to unmarshal event envelope", zap.Error(err))
			session.MarkMessage(msg, "")
			continue
		}
		if err := h.handler.Handle(session.Context(), envelope); err != nil {
			h.logger.Error("event handler error", zap.String("event_type", envelope.EventType), zap.Error(err))
		}
		session.MarkMessage(msg, "")
	}
	return nil
}
