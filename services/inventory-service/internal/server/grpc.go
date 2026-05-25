package server

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/pulse-platform/inventory-service/gen/pb"
	"github.com/pulse-platform/inventory-service/internal/domain"
	"github.com/pulse-platform/inventory-service/internal/repository"
)

// Producer defines the event publishing interface required by InventoryServer.
type Producer interface {
	PublishInventoryReserved(ctx context.Context, orderID, reservationID, tenantID string) error
	PublishInventoryFailed(ctx context.Context, orderID, reason, tenantID string) error
}

// InventoryServer implements the gRPC InventoryServiceServer.
type InventoryServer struct {
	pb.UnimplementedInventoryServiceServer
	repo     *repository.InventoryRepository
	producer Producer
	logger   *zap.Logger
}

// NewInventoryServer creates a new InventoryServer with the given dependencies.
func NewInventoryServer(repo *repository.InventoryRepository, producer Producer, logger *zap.Logger) *InventoryServer {
	return &InventoryServer{repo: repo, producer: producer, logger: logger}
}

// CheckStock returns the current availability and stock level for a product.
func (s *InventoryServer) CheckStock(ctx context.Context, req *pb.CheckStockRequest) (*pb.CheckStockResponse, error) {
	item, err := s.repo.FindByProductID(ctx, req.GetProductID())
	if err != nil {
		s.logger.Error("check stock: find product", zap.String("product_id", req.GetProductID()), zap.Error(err))
		return nil, status.Errorf(codes.NotFound, "product not found: %s", req.GetProductID())
	}
	available := item.Available() >= req.GetQuantity()
	return &pb.CheckStockResponse{
		Available:  available,
		StockLevel: item.Available(),
	}, nil
}

// ReserveStock attempts to reserve stock for an order, publishing the outcome to Kafka.
func (s *InventoryServer) ReserveStock(ctx context.Context, req *pb.ReserveStockRequest) (*pb.ReserveStockResponse, error) {
	reservation := domain.NewReservation(req.GetOrderID(), req.GetProductID(), req.GetQuantity())

	err := s.repo.CreateReservation(ctx, reservation)
	if err != nil {
		s.logger.Warn("reserve stock failed", zap.String("order_id", req.GetOrderID()), zap.Error(err))
		if pubErr := s.producer.PublishInventoryFailed(ctx, req.GetOrderID(), err.Error(), ""); pubErr != nil {
			s.logger.Error("publish inventory.failed", zap.Error(pubErr))
		}
		return &pb.ReserveStockResponse{Success: false}, nil
	}

	if pubErr := s.producer.PublishInventoryReserved(ctx, req.GetOrderID(), reservation.ID, ""); pubErr != nil {
		s.logger.Error("publish inventory.reserved", zap.Error(pubErr))
	}

	return &pb.ReserveStockResponse{
		Success:       true,
		ReservationID: reservation.ID,
	}, nil
}

// ReleaseStock releases a previously created reservation.
func (s *InventoryServer) ReleaseStock(ctx context.Context, req *pb.ReleaseStockRequest) (*pb.ReleaseStockResponse, error) {
	if err := s.repo.ReleaseReservation(ctx, req.GetReservationID()); err != nil {
		s.logger.Error("release stock", zap.String("reservation_id", req.GetReservationID()), zap.Error(err))
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("release reservation: %v", err))
	}
	return &pb.ReleaseStockResponse{Success: true}, nil
}
