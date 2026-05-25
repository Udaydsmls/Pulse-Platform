package handler

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	"github.com/pulse-platform/inventory-service/internal/domain"
	"github.com/pulse-platform/inventory-service/internal/kafka"
	"github.com/pulse-platform/inventory-service/internal/repository"
)

// OrderCreatedPayload is the expected payload for order.created events.
type OrderCreatedPayload struct {
	OrderID   string `json:"order_id"`
	ProductID string `json:"product_id"`
	Quantity  int32  `json:"quantity"`
	TenantID  string `json:"tenant_id"`
}

// OrderCreatedHandler processes order.created events by attempting stock reservation.
type OrderCreatedHandler struct {
	repo     *repository.InventoryRepository
	producer *kafka.KafkaProducer
	logger   *zap.Logger
}

// NewOrderCreatedHandler creates a new OrderCreatedHandler.
func NewOrderCreatedHandler(repo *repository.InventoryRepository, producer *kafka.KafkaProducer, logger *zap.Logger) *OrderCreatedHandler {
	return &OrderCreatedHandler{repo: repo, producer: producer, logger: logger}
}

// Handle processes the EventEnvelope, acting only on order.created events.
func (h *OrderCreatedHandler) Handle(ctx context.Context, envelope kafka.EventEnvelope) error {
	if envelope.EventType != "order.created" {
		return nil
	}

	var payload OrderCreatedPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal order.created payload: %w", err)
	}

	reservation := domain.NewReservation(payload.OrderID, payload.ProductID, payload.Quantity)
	if err := h.repo.CreateReservation(ctx, reservation); err != nil {
		h.logger.Warn("reservation failed",
			zap.String("order_id", payload.OrderID),
			zap.String("product_id", payload.ProductID),
			zap.Error(err),
		)
		if pubErr := h.producer.PublishInventoryFailed(ctx, payload.OrderID, err.Error(), payload.TenantID); pubErr != nil {
			return fmt.Errorf("publish inventory.failed: %w", pubErr)
		}
		return nil
	}

	h.logger.Info("stock reserved",
		zap.String("order_id", payload.OrderID),
		zap.String("reservation_id", reservation.ID),
	)

	if err := h.producer.PublishInventoryReserved(ctx, payload.OrderID, reservation.ID, payload.TenantID); err != nil {
		return fmt.Errorf("publish inventory.reserved: %w", err)
	}
	return nil
}
