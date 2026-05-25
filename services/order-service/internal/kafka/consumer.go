package kafka

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/IBM/sarama"
	"go.uber.org/zap"
)

// EventHandler processes a decoded EventEnvelope from a Kafka topic.
type EventHandler interface {
	Handle(ctx context.Context, event EventEnvelope) error
}

// KafkaConsumer wraps a sarama ConsumerGroup to consume order-related events.
type KafkaConsumer struct {
	group   sarama.ConsumerGroup
	topics  []string
	handler EventHandler
	logger  *zap.Logger
}

// NewConsumer creates a KafkaConsumer that subscribes to the given topics using the specified consumer group.
func NewConsumer(brokers []string, groupID string, topics []string, handler EventHandler, logger *zap.Logger) (*KafkaConsumer, error) {
	cfg := sarama.NewConfig()
	cfg.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRoundRobin()}
	cfg.Consumer.Offsets.Initial = sarama.OffsetNewest

	group, err := sarama.NewConsumerGroup(brokers, groupID, cfg)
	if err != nil {
		return nil, fmt.Errorf("create consumer group: %w", err)
	}

	return &KafkaConsumer{
		group:   group,
		topics:  topics,
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
	if err := c.group.Close(); err != nil {
		return fmt.Errorf("close consumer group: %w", err)
	}
	return nil
}

type consumerGroupHandler struct {
	handler EventHandler
	logger  *zap.Logger
}

func (h *consumerGroupHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *consumerGroupHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case msg, ok := <-claim.Messages():
			if !ok {
				return nil
			}

			var envelope EventEnvelope
			if err := json.Unmarshal(msg.Value, &envelope); err != nil {
				h.logger.Error("unmarshal event envelope",
					zap.String("topic", msg.Topic),
					zap.Error(err),
				)
				session.MarkMessage(msg, "")
				continue
			}

			if err := h.handler.Handle(session.Context(), envelope); err != nil {
				h.logger.Error("handle event",
					zap.String("event_type", envelope.EventType),
					zap.Error(err),
				)
			}

			session.MarkMessage(msg, "")

		case <-session.Context().Done():
			return nil
		}
	}
}
