package handler

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	"github.com/pulse-platform/payment-service/internal/domain"
	"github.com/pulse-platform/payment-service/internal/kafka"
	"github.com/pulse-platform/payment-service/internal/repository"
	"github.com/pulse-platform/payment-service/internal/stripe"
)

// InventoryReservedPayload is the expected payload for inventory.reserved events.
type InventoryReservedPayload struct {
	OrderID       string  `json:"order_id"`
	ReservationID string  `json:"reservation_id"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	TenantID      string  `json:"tenant_id"`
	UserID        string  `json:"user_id"`
}

// InventoryReservedHandler processes inventory.reserved events by initiating payment.
type InventoryReservedHandler struct {
	repo     *repository.PaymentRepository
	stripe   stripe.StripeClient
	producer *kafka.KafkaProducer
	logger   *zap.Logger
}

// NewInventoryReservedHandler creates a new InventoryReservedHandler.
func NewInventoryReservedHandler(
	repo *repository.PaymentRepository,
	stripeClient stripe.StripeClient,
	producer *kafka.KafkaProducer,
	logger *zap.Logger,
) *InventoryReservedHandler {
	return &InventoryReservedHandler{
		repo:     repo,
		stripe:   stripeClient,
		producer: producer,
		logger:   logger,
	}
}

// Handle processes the EventEnvelope, acting only on inventory.reserved events.
func (h *InventoryReservedHandler) Handle(ctx context.Context, envelope kafka.EventEnvelope) error {
	if envelope.EventType != "inventory.reserved" {
		return nil
	}

	var payload InventoryReservedPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal inventory.reserved payload: %w", err)
	}

	payment := domain.NewPayment(payload.OrderID, payload.UserID, payload.Amount, payload.Currency)
	if err := h.repo.Create(ctx, payment); err != nil {
		return fmt.Errorf("create payment record: %w", err)
	}

	result, err := h.stripe.Charge(ctx, &stripe.ChargeRequest{
		Amount:   payload.Amount,
		Currency: payload.Currency,
		Token:    "tok_visa",
		OrderID:  payload.OrderID,
	})
	if err != nil {
		h.logger.Error("stripe charge error", zap.String("order_id", payload.OrderID), zap.Error(err))
		payment.Fail(err.Error())
		_ = h.repo.UpdateStatus(ctx, payment.ID, string(payment.Status), "")
		if pubErr := h.producer.PublishPaymentFailed(ctx, payload.OrderID, err.Error(), payload.TenantID); pubErr != nil {
			return fmt.Errorf("publish payment.failed: %w", pubErr)
		}
		return nil
	}

	if !result.Success {
		payment.Fail("charge declined")
		_ = h.repo.UpdateStatus(ctx, payment.ID, string(payment.Status), "")
		if pubErr := h.producer.PublishPaymentFailed(ctx, payload.OrderID, "charge declined", payload.TenantID); pubErr != nil {
			return fmt.Errorf("publish payment.failed: %w", pubErr)
		}
		return nil
	}

	payment.Complete(result.TransactionID)
	if err := h.repo.UpdateStatus(ctx, payment.ID, string(payment.Status), payment.TransactionID); err != nil {
		h.logger.Error("update payment status", zap.String("payment_id", payment.ID), zap.Error(err))
	}

	h.logger.Info("payment confirmed",
		zap.String("order_id", payload.OrderID),
		zap.String("transaction_id", result.TransactionID),
	)

	if err := h.producer.PublishPaymentConfirmed(ctx, payload.OrderID, result.TransactionID, payload.TenantID); err != nil {
		return fmt.Errorf("publish payment.confirmed: %w", err)
	}
	return nil
}
