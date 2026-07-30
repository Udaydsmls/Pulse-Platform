package main

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/pulse-platform/order-service/gen/pb"
)

// OrderServer implements the OrderService gRPC API.
type OrderServer struct {
	pb.UnimplementedOrderServiceServer
	db       *DB
	producer *Producer
	saga     *Saga
}

// CreateOrder saves a pending order and kicks off the saga by publishing
// order.created.
func (s *OrderServer) CreateOrder(ctx context.Context, req *pb.CreateOrderRequest) (*pb.CreateOrderResponse, error) {
	items := make([]OrderItem, 0, len(req.GetItems()))
	for _, item := range req.GetItems() {
		items = append(items, OrderItem{
			ProductID: item.GetProductId(),
			Quantity:  item.GetQuantity(),
			UnitPrice: item.GetUnitPrice(),
		})
	}

	order := NewOrder(uuid.NewString(), req.GetUserId(), req.GetEmail(), items)
	if err := order.Validate(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if err := s.db.Insert(ctx, order); err != nil {
		log.Printf("insert order: %v", err)
		return nil, status.Error(codes.Internal, "could not create order")
	}

	// The order row is already committed, so a publish failure would strand it
	// in pending forever. Report it rather than returning a success the saga
	// will never follow up on.
	err := s.producer.Publish(ctx, Event{
		Type:    "order.created",
		OrderID: order.ID,
		UserID:  order.UserID,
		Email:   order.Email,
		Items:   order.Items,
		Total:   order.Total,
	})
	if err != nil {
		log.Printf("publish order.created for %s: %v", order.ID, err)
		return nil, status.Error(codes.Internal, "order saved but could not be submitted")
	}

	return &pb.CreateOrderResponse{OrderId: order.ID, Status: order.Status}, nil
}

// CancelOrder cancels an order at the customer's request. It reuses the saga's
// Cancel step, so the same compensating order.cancelled event is published and
// inventory-service releases the stock.
func (s *OrderServer) CancelOrder(ctx context.Context, req *pb.CancelOrderRequest) (*pb.CancelOrderResponse, error) {
	if req.GetOrderId() == "" || req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "order_id and user_id are required")
	}

	order, err := s.db.FindByID(ctx, req.GetOrderId())
	if err != nil {
		return nil, status.Error(codes.NotFound, "order not found")
	}

	if order.UserID != req.GetUserId() {
		return nil, status.Error(codes.PermissionDenied, "order belongs to another user")
	}

	if !order.CanCancel() {
		return nil, status.Errorf(codes.FailedPrecondition, "order is already %s", order.Status)
	}

	if err := s.saga.Cancel(ctx, order.ID, "cancelled by customer"); err != nil {
		log.Printf("cancel order %s: %v", order.ID, err)
		return nil, status.Error(codes.Internal, "could not cancel order")
	}

	return &pb.CancelOrderResponse{Success: true}, nil
}

// GetOrder returns a single order.
func (s *OrderServer) GetOrder(ctx context.Context, req *pb.GetOrderRequest) (*pb.GetOrderResponse, error) {
	if req.GetOrderId() == "" {
		return nil, status.Error(codes.InvalidArgument, "order_id is required")
	}

	order, err := s.db.FindByID(ctx, req.GetOrderId())
	if err != nil {
		return nil, status.Error(codes.NotFound, "order not found")
	}

	items := make([]*pb.OrderItem, 0, len(order.Items))
	for _, item := range order.Items {
		items = append(items, &pb.OrderItem{
			ProductId: item.ProductID,
			Quantity:  item.Quantity,
			UnitPrice: item.UnitPrice,
		})
	}

	return &pb.GetOrderResponse{
		OrderId:   order.ID,
		UserId:    order.UserID,
		Status:    order.Status,
		Items:     items,
		Total:     order.Total,
		CreatedAt: order.CreatedAt.Format(time.RFC3339),
	}, nil
}
