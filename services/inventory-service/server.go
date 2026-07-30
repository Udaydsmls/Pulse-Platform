package main

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/pulse-platform/inventory-service/gen/pb"
)

// InventoryServer implements the InventoryService gRPC API. Reservations happen
// over Kafka in the saga; this is only the synchronous stock lookup.
type InventoryServer struct {
	pb.UnimplementedInventoryServiceServer
	db *DB
}

// CheckStock reports whether a product has enough units available.
func (s *InventoryServer) CheckStock(ctx context.Context, req *pb.CheckStockRequest) (*pb.CheckStockResponse, error) {
	if req.GetProductId() == "" {
		return nil, status.Error(codes.InvalidArgument, "product_id is required")
	}

	item, err := s.db.FindStock(ctx, req.GetProductId())
	if err != nil {
		return nil, status.Error(codes.NotFound, "product not found")
	}

	return &pb.CheckStockResponse{
		Available:  item.Available() >= req.GetQuantity(),
		StockLevel: item.Available(),
	}, nil
}
