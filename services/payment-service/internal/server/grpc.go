package server

import (
	"context"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/pulse-platform/payment-service/gen/pb"
	"github.com/pulse-platform/payment-service/internal/domain"
	"github.com/pulse-platform/payment-service/internal/repository"
	"github.com/pulse-platform/payment-service/internal/stripe"
)

// Producer defines the event publishing interface required by PaymentServer.
type Producer interface {
	PublishPaymentConfirmed(ctx context.Context, orderID, transactionID, tenantID string) error
	PublishPaymentFailed(ctx context.Context, orderID, reason, tenantID string) error
}

// PaymentServer implements the gRPC PaymentServiceServer.
type PaymentServer struct {
	pb.UnimplementedPaymentServiceServer
	repo     *repository.PaymentRepository
	stripe   stripe.StripeClient
	producer Producer
	logger   *zap.Logger
}

// NewPaymentServer creates a new PaymentServer with the given dependencies.
func NewPaymentServer(repo *repository.PaymentRepository, stripeClient stripe.StripeClient, producer Producer, logger *zap.Logger) *PaymentServer {
	return &PaymentServer{repo: repo, stripe: stripeClient, producer: producer, logger: logger}
}

// ProcessPayment creates a payment record, charges via Stripe, and returns the result.
func (s *PaymentServer) ProcessPayment(ctx context.Context, req *pb.ProcessPaymentRequest) (*pb.ProcessPaymentResponse, error) {
	payment := domain.NewPayment(req.GetOrderID(), req.GetUserID(), req.GetAmount(), req.GetCurrency())
	if err := s.repo.Create(ctx, payment); err != nil {
		s.logger.Error("create payment", zap.String("order_id", req.GetOrderID()), zap.Error(err))
		return nil, status.Errorf(codes.Internal, "create payment: %v", err)
	}

	result, err := s.stripe.Charge(ctx, &stripe.ChargeRequest{
		Amount:   req.GetAmount(),
		Currency: req.GetCurrency(),
		Token:    req.GetPaymentMethodToken(),
		OrderID:  req.GetOrderID(),
	})
	if err != nil {
		payment.Fail(err.Error())
		_ = s.repo.UpdateStatus(ctx, payment.ID, string(payment.Status), "")
		return nil, status.Errorf(codes.Internal, "charge payment: %v", err)
	}

	if !result.Success {
		payment.Fail("charge declined")
		_ = s.repo.UpdateStatus(ctx, payment.ID, string(payment.Status), "")
		if pubErr := s.producer.PublishPaymentFailed(ctx, req.GetOrderID(), "charge declined", ""); pubErr != nil {
			s.logger.Error("publish payment.failed", zap.Error(pubErr))
		}
		return &pb.ProcessPaymentResponse{Success: false, Status: string(payment.Status)}, nil
	}

	payment.Complete(result.TransactionID)
	if err := s.repo.UpdateStatus(ctx, payment.ID, string(payment.Status), payment.TransactionID); err != nil {
		s.logger.Error("update payment status", zap.String("payment_id", payment.ID), zap.Error(err))
	}

	if pubErr := s.producer.PublishPaymentConfirmed(ctx, req.GetOrderID(), result.TransactionID, ""); pubErr != nil {
		s.logger.Error("publish payment.confirmed", zap.Error(pubErr))
	}

	return &pb.ProcessPaymentResponse{
		Success:       true,
		TransactionID: result.TransactionID,
		Status:        string(payment.Status),
	}, nil
}
