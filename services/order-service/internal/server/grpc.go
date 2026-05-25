package server

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/pulse-platform/order-service/gen/pb"
	"github.com/pulse-platform/order-service/internal/domain"
	"github.com/pulse-platform/order-service/internal/kafka"
	"github.com/pulse-platform/order-service/internal/repository"
)

// OrderServer implements the gRPC OrderServiceServer interface.
type OrderServer struct {
	pb.UnimplementedOrderServiceServer
	repo     *repository.OrderRepository
	producer *kafka.KafkaProducer
	logger   *zap.Logger
}

// NewOrderServer constructs an OrderServer with the required dependencies.
func NewOrderServer(repo *repository.OrderRepository, producer *kafka.KafkaProducer, logger *zap.Logger) *OrderServer {
	return &OrderServer{repo: repo, producer: producer, logger: logger}
}

// CreateOrder creates a new order and publishes an order.created event.
func (s *OrderServer) CreateOrder(ctx context.Context, req *pb.CreateOrderRequest) (*pb.CreateOrderResponse, error) {
	if req.UserId == "" || req.TenantId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id and tenant_id are required")
	}
	if len(req.Items) == 0 {
		return nil, status.Error(codes.InvalidArgument, "order must contain at least one item")
	}

	items := make([]domain.OrderItem, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, domain.OrderItem{
			ProductID: it.ProductId,
			Quantity:  int(it.Quantity),
			UnitPrice: it.UnitPrice,
		})
	}

	order := domain.NewOrder(req.UserId, req.TenantId, items)
	order.ID = uuid.New().String()

	if err := order.Validate(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if err := s.repo.Create(ctx, order); err != nil {
		s.logger.Error("create order", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to create order")
	}

	if err := s.producer.PublishOrderCreated(ctx, order.ID, order.UserID, order.TenantID, order.Total); err != nil {
		s.logger.Warn("publish order.created event", zap.String("order_id", order.ID), zap.Error(err))
	}

	return &pb.CreateOrderResponse{
		OrderId: order.ID,
		Status:  string(order.Status),
	}, nil
}

// CancelOrder transitions an order to cancelled status.
func (s *OrderServer) CancelOrder(ctx context.Context, req *pb.CancelOrderRequest) (*pb.CancelOrderResponse, error) {
	if req.OrderId == "" || req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "order_id and user_id are required")
	}

	order, err := s.repo.FindByID(ctx, req.OrderId)
	if err != nil {
		return nil, status.Error(codes.NotFound, "order not found")
	}

	if order.UserID != req.UserId {
		return nil, status.Error(codes.PermissionDenied, "order does not belong to user")
	}

	if order.Status == domain.OrderStatusCancelled || order.Status == domain.OrderStatusFailed {
		return nil, status.Error(codes.FailedPrecondition, fmt.Sprintf("order is already %s", order.Status))
	}

	order.Cancel()
	if err := s.repo.UpdateStatus(ctx, order.ID, domain.OrderStatusCancelled); err != nil {
		s.logger.Error("update order status", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to cancel order")
	}

	if err := s.producer.PublishOrderCancelled(ctx, order.ID, order.TenantID); err != nil {
		s.logger.Warn("publish order.cancelled event", zap.String("order_id", order.ID), zap.Error(err))
	}

	return &pb.CancelOrderResponse{Success: true}, nil
}

// GetOrder retrieves an order by its ID.
func (s *OrderServer) GetOrder(ctx context.Context, req *pb.GetOrderRequest) (*pb.GetOrderResponse, error) {
	if req.OrderId == "" {
		return nil, status.Error(codes.InvalidArgument, "order_id is required")
	}

	order, err := s.repo.FindByID(ctx, req.OrderId)
	if err != nil {
		return nil, status.Error(codes.NotFound, "order not found")
	}

	pbItems := make([]*pb.OrderItem, 0, len(order.Items))
	for _, item := range order.Items {
		pbItems = append(pbItems, &pb.OrderItem{
			ProductId: item.ProductID,
			Quantity:  int32(item.Quantity),
			UnitPrice: item.UnitPrice,
		})
	}

	return &pb.GetOrderResponse{
		OrderId:   order.ID,
		UserId:    order.UserID,
		Status:    string(order.Status),
		Items:     pbItems,
		Total:     order.Total,
		CreatedAt: order.CreatedAt.String(),
	}, nil
}
