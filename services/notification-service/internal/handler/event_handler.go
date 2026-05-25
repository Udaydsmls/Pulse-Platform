package handler

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	"github.com/pulse-platform/notification-service/internal/domain"
	"github.com/pulse-platform/notification-service/internal/kafka"
	"github.com/pulse-platform/notification-service/internal/notification"
	"github.com/pulse-platform/notification-service/internal/repository"
)

// NotificationHandler routes domain events to the appropriate notification channels.
type NotificationHandler struct {
	emailSender notification.EmailSender
	smsSender   notification.SMSSender
	repo        *repository.NotificationRepository
	logger      *zap.Logger
}

// NewNotificationHandler creates a new NotificationHandler with the given dependencies.
func NewNotificationHandler(
	emailSender notification.EmailSender,
	smsSender notification.SMSSender,
	repo *repository.NotificationRepository,
	logger *zap.Logger,
) *NotificationHandler {
	return &NotificationHandler{
		emailSender: emailSender,
		smsSender:   smsSender,
		repo:        repo,
		logger:      logger,
	}
}

// Handle dispatches the event to the correct notification flow based on EventType.
func (h *NotificationHandler) Handle(ctx context.Context, envelope kafka.EventEnvelope) error {
	switch envelope.EventType {
	case "user.created":
		return h.handleUserCreated(ctx, envelope)
	case "order.created":
		return h.handleOrderCreated(ctx, envelope)
	case "order.confirmed":
		return h.handleOrderConfirmed(ctx, envelope)
	case "order.cancelled":
		return h.handleOrderCancelled(ctx, envelope)
	case "payment.confirmed":
		return h.handlePaymentConfirmed(ctx, envelope)
	case "payment.failed":
		return h.handlePaymentFailed(ctx, envelope)
	default:
		return nil
	}
}

func (h *NotificationHandler) handleUserCreated(ctx context.Context, envelope kafka.EventEnvelope) error {
	var p struct {
		UserID string `json:"user_id"`
		Email  string `json:"email"`
	}
	if err := json.Unmarshal(envelope.Payload, &p); err != nil {
		return fmt.Errorf("unmarshal user.created payload: %w", err)
	}
	if err := h.emailSender.Send(ctx, p.Email, "Welcome to Pulse Platform", "Thank you for joining us!"); err != nil {
		h.logger.Error("send welcome email", zap.String("user_id", p.UserID), zap.Error(err))
	}
	return h.saveLog(ctx, p.UserID, envelope.EventType, "email", string(envelope.Payload))
}

func (h *NotificationHandler) handleOrderCreated(ctx context.Context, envelope kafka.EventEnvelope) error {
	var p struct {
		OrderID string `json:"order_id"`
		UserID  string `json:"user_id"`
		Email   string `json:"email"`
	}
	if err := json.Unmarshal(envelope.Payload, &p); err != nil {
		return fmt.Errorf("unmarshal order.created payload: %w", err)
	}
	subject := fmt.Sprintf("Order %s received", p.OrderID)
	body := fmt.Sprintf("Your order %s has been received and is being processed.", p.OrderID)
	if err := h.emailSender.Send(ctx, p.Email, subject, body); err != nil {
		h.logger.Error("send order confirmation email", zap.String("order_id", p.OrderID), zap.Error(err))
	}
	return h.saveLog(ctx, p.UserID, envelope.EventType, "email", string(envelope.Payload))
}

func (h *NotificationHandler) handleOrderConfirmed(ctx context.Context, envelope kafka.EventEnvelope) error {
	var p struct {
		OrderID string `json:"order_id"`
		UserID  string `json:"user_id"`
		Email   string `json:"email"`
	}
	if err := json.Unmarshal(envelope.Payload, &p); err != nil {
		return fmt.Errorf("unmarshal order.confirmed payload: %w", err)
	}
	subject := fmt.Sprintf("Order %s confirmed", p.OrderID)
	body := fmt.Sprintf("Great news! Your order %s has been confirmed.", p.OrderID)
	if err := h.emailSender.Send(ctx, p.Email, subject, body); err != nil {
		h.logger.Error("send order confirmed email", zap.String("order_id", p.OrderID), zap.Error(err))
	}
	return h.saveLog(ctx, p.UserID, envelope.EventType, "email", string(envelope.Payload))
}

func (h *NotificationHandler) handleOrderCancelled(ctx context.Context, envelope kafka.EventEnvelope) error {
	var p struct {
		OrderID string `json:"order_id"`
		UserID  string `json:"user_id"`
		Email   string `json:"email"`
	}
	if err := json.Unmarshal(envelope.Payload, &p); err != nil {
		return fmt.Errorf("unmarshal order.cancelled payload: %w", err)
	}
	subject := fmt.Sprintf("Order %s cancelled", p.OrderID)
	body := fmt.Sprintf("Your order %s has been cancelled.", p.OrderID)
	if err := h.emailSender.Send(ctx, p.Email, subject, body); err != nil {
		h.logger.Error("send cancellation email", zap.String("order_id", p.OrderID), zap.Error(err))
	}
	return h.saveLog(ctx, p.UserID, envelope.EventType, "email", string(envelope.Payload))
}

func (h *NotificationHandler) handlePaymentConfirmed(ctx context.Context, envelope kafka.EventEnvelope) error {
	var p struct {
		OrderID       string `json:"order_id"`
		UserID        string `json:"user_id"`
		Email         string `json:"email"`
		TransactionID string `json:"transaction_id"`
	}
	if err := json.Unmarshal(envelope.Payload, &p); err != nil {
		return fmt.Errorf("unmarshal payment.confirmed payload: %w", err)
	}
	subject := fmt.Sprintf("Payment receipt for order %s", p.OrderID)
	body := fmt.Sprintf("Payment confirmed for order %s. Transaction ID: %s", p.OrderID, p.TransactionID)
	if err := h.emailSender.Send(ctx, p.Email, subject, body); err != nil {
		h.logger.Error("send payment receipt email", zap.String("order_id", p.OrderID), zap.Error(err))
	}
	return h.saveLog(ctx, p.UserID, envelope.EventType, "email", string(envelope.Payload))
}

func (h *NotificationHandler) handlePaymentFailed(ctx context.Context, envelope kafka.EventEnvelope) error {
	var p struct {
		OrderID string `json:"order_id"`
		UserID  string `json:"user_id"`
		Email   string `json:"email"`
		Reason  string `json:"reason"`
	}
	if err := json.Unmarshal(envelope.Payload, &p); err != nil {
		return fmt.Errorf("unmarshal payment.failed payload: %w", err)
	}
	subject := fmt.Sprintf("Payment failed for order %s", p.OrderID)
	body := fmt.Sprintf("Unfortunately, the payment for order %s failed. Reason: %s", p.OrderID, p.Reason)
	if err := h.emailSender.Send(ctx, p.Email, subject, body); err != nil {
		h.logger.Error("send payment failure email", zap.String("order_id", p.OrderID), zap.Error(err))
	}
	return h.saveLog(ctx, p.UserID, envelope.EventType, "email", string(envelope.Payload))
}

func (h *NotificationHandler) saveLog(ctx context.Context, userID, eventType, channel, payload string) error {
	log := domain.NewNotificationLog(userID, eventType, channel, payload)
	if err := h.repo.Save(ctx, log); err != nil {
		h.logger.Error("save notification log", zap.String("user_id", userID), zap.Error(err))
		return fmt.Errorf("save notification log: %w", err)
	}
	return nil
}
