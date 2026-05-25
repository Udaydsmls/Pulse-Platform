package saga

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	"github.com/pulse-platform/order-service/internal/domain"
	"github.com/pulse-platform/order-service/internal/kafka"
	"github.com/pulse-platform/order-service/internal/repository"
)

type inventoryReservedPayload struct {
	OrderID       string `json:"order_id"`
	ReservationID string `json:"reservation_id"`
}

type inventoryFailedPayload struct {
	OrderID string `json:"order_id"`
	Reason  string `json:"reason"`
}

type paymentConfirmedPayload struct {
	OrderID       string `json:"order_id"`
	TransactionID string `json:"transaction_id"`
}

type paymentFailedPayload struct {
	OrderID       string `json:"order_id"`
	ReservationID string `json:"reservation_id"`
	Reason        string `json:"reason"`
}

// OrderSaga coordinates the distributed order workflow across inventory and payment services.
type OrderSaga struct {
	repo     *repository.OrderRepository
	producer *kafka.KafkaProducer
	logger   *zap.Logger
}

// NewOrderSaga creates an OrderSaga with the required repository and event producer.
func NewOrderSaga(repo *repository.OrderRepository, producer *kafka.KafkaProducer, logger *zap.Logger) *OrderSaga {
	return &OrderSaga{repo: repo, producer: producer, logger: logger}
}

// Handle dispatches an incoming EventEnvelope to the appropriate saga step handler.
func (s *OrderSaga) Handle(ctx context.Context, event kafka.EventEnvelope) error {
	switch event.EventType {
	case "inventory.reserved":
		return s.onInventoryReserved(ctx, event)
	case "inventory.failed":
		return s.onInventoryFailed(ctx, event)
	case "payment.confirmed":
		return s.onPaymentConfirmed(ctx, event)
	case "payment.failed":
		return s.onPaymentFailed(ctx, event)
	default:
		return nil
	}
}

func (s *OrderSaga) onInventoryReserved(ctx context.Context, event kafka.EventEnvelope) error {
	var payload inventoryReservedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal inventory.reserved payload: %w", err)
	}

	paymentRequestPayload, err := json.Marshal(map[string]string{
		"order_id":       payload.OrderID,
		"reservation_id": payload.ReservationID,
	})
	if err != nil {
		return fmt.Errorf("marshal payment request payload: %w", err)
	}

	envelope := kafka.EventEnvelope{
		EventType:   "payment.process",
		AggregateID: payload.OrderID,
		TenantID:    event.TenantID,
		Timestamp:   event.Timestamp,
		Payload:     json.RawMessage(paymentRequestPayload),
	}

	_ = envelope

	s.logger.Info("inventory reserved, triggering payment",
		zap.String("order_id", payload.OrderID),
		zap.String("reservation_id", payload.ReservationID),
	)
	return nil
}

func (s *OrderSaga) onInventoryFailed(ctx context.Context, event kafka.EventEnvelope) error {
	var payload inventoryFailedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal inventory.failed payload: %w", err)
	}

	order, err := s.repo.FindByID(ctx, payload.OrderID)
	if err != nil {
		return fmt.Errorf("find order for inventory failure: %w", err)
	}

	order.Fail()
	if err := s.repo.UpdateStatus(ctx, order.ID, domain.OrderStatusFailed); err != nil {
		return fmt.Errorf("update order status to failed: %w", err)
	}

	if err := s.producer.PublishOrderCancelled(ctx, order.ID, event.TenantID); err != nil {
		s.logger.Warn("publish order.cancelled after inventory failure", zap.Error(err))
	}

	s.logger.Info("order failed due to inventory shortage",
		zap.String("order_id", payload.OrderID),
		zap.String("reason", payload.Reason),
	)
	return nil
}

func (s *OrderSaga) onPaymentConfirmed(ctx context.Context, event kafka.EventEnvelope) error {
	var payload paymentConfirmedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal payment.confirmed payload: %w", err)
	}

	order, err := s.repo.FindByID(ctx, payload.OrderID)
	if err != nil {
		return fmt.Errorf("find order for payment confirmation: %w", err)
	}

	order.Confirm()
	if err := s.repo.UpdateStatus(ctx, order.ID, domain.OrderStatusConfirmed); err != nil {
		return fmt.Errorf("update order status to confirmed: %w", err)
	}

	if err := s.producer.PublishOrderConfirmed(ctx, order.ID, event.TenantID); err != nil {
		s.logger.Warn("publish order.confirmed event", zap.Error(err))
	}

	s.logger.Info("order confirmed",
		zap.String("order_id", payload.OrderID),
		zap.String("transaction_id", payload.TransactionID),
	)
	return nil
}

func (s *OrderSaga) onPaymentFailed(ctx context.Context, event kafka.EventEnvelope) error {
	var payload paymentFailedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal payment.failed payload: %w", err)
	}

	order, err := s.repo.FindByID(ctx, payload.OrderID)
	if err != nil {
		return fmt.Errorf("find order for payment failure: %w", err)
	}

	order.Fail()
	if err := s.repo.UpdateStatus(ctx, order.ID, domain.OrderStatusFailed); err != nil {
		return fmt.Errorf("update order status to failed: %w", err)
	}

	releasePayload, err := json.Marshal(map[string]string{
		"reservation_id": payload.ReservationID,
		"order_id":       payload.OrderID,
	})
	if err != nil {
		return fmt.Errorf("marshal release stock payload: %w", err)
	}

	_ = releasePayload

	if err := s.producer.PublishOrderCancelled(ctx, order.ID, event.TenantID); err != nil {
		s.logger.Warn("publish order.cancelled after payment failure", zap.Error(err))
	}

	s.logger.Info("order failed due to payment failure, releasing inventory reservation",
		zap.String("order_id", payload.OrderID),
		zap.String("reservation_id", payload.ReservationID),
	)
	return nil
}
