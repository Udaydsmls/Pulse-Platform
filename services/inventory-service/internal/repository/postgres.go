package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pulse-platform/inventory-service/internal/domain"
)

// InventoryRepository handles persistence for stock items and reservations.
type InventoryRepository struct {
	pool *pgxpool.Pool
}

// NewInventoryRepository creates a new InventoryRepository backed by the given connection pool.
func NewInventoryRepository(pool *pgxpool.Pool) *InventoryRepository {
	return &InventoryRepository{pool: pool}
}

// CreateTable creates the stock_items and reservations tables if they do not exist.
func (r *InventoryRepository) CreateTable(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS stock_items (
			id          TEXT PRIMARY KEY,
			product_id  TEXT UNIQUE NOT NULL,
			stock_level INT NOT NULL DEFAULT 0,
			reserved    INT NOT NULL DEFAULT 0,
			updated_at  TIMESTAMPTZ NOT NULL
		);
		CREATE TABLE IF NOT EXISTS reservations (
			id         TEXT PRIMARY KEY,
			order_id   TEXT NOT NULL,
			product_id TEXT NOT NULL,
			quantity   INT NOT NULL,
			status     TEXT NOT NULL DEFAULT 'active',
			created_at TIMESTAMPTZ NOT NULL
		);
	`)
	if err != nil {
		return fmt.Errorf("create tables: %w", err)
	}
	return nil
}

// FindByProductID retrieves a StockItem by its product ID.
func (r *InventoryRepository) FindByProductID(ctx context.Context, productID string) (*domain.StockItem, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, product_id, stock_level, reserved, updated_at FROM stock_items WHERE product_id = $1`,
		productID,
	)
	item := &domain.StockItem{}
	if err := row.Scan(&item.ID, &item.ProductID, &item.StockLevel, &item.Reserved, &item.UpdatedAt); err != nil {
		return nil, fmt.Errorf("find stock item by product id: %w", err)
	}
	return item, nil
}

// UpdateStockLevel sets the stock_level for the given product.
func (r *InventoryRepository) UpdateStockLevel(ctx context.Context, productID string, level int32) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE stock_items SET stock_level = $1, updated_at = NOW() WHERE product_id = $2`,
		level, productID,
	)
	if err != nil {
		return fmt.Errorf("update stock level: %w", err)
	}
	return nil
}

// CreateReservation persists a new Reservation atomically with the stock decrement.
func (r *InventoryRepository) CreateReservation(ctx context.Context, res *domain.Reservation) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var stockLevel, reserved int32
	err = tx.QueryRow(ctx,
		`SELECT stock_level, reserved FROM stock_items WHERE product_id = $1 FOR UPDATE`,
		res.ProductID,
	).Scan(&stockLevel, &reserved)
	if err != nil {
		return fmt.Errorf("lock stock item: %w", err)
	}

	if stockLevel-reserved < res.Quantity {
		return fmt.Errorf("insufficient stock: available %d, requested %d", stockLevel-reserved, res.Quantity)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO reservations (id, order_id, product_id, quantity, status, created_at)
		 VALUES ($1, $2, $3, $4, 'active', $5)`,
		res.ID, res.OrderID, res.ProductID, res.Quantity, res.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert reservation: %w", err)
	}

	_, err = tx.Exec(ctx,
		`UPDATE stock_items SET reserved = reserved + $1, updated_at = NOW() WHERE product_id = $2`,
		res.Quantity, res.ProductID,
	)
	if err != nil {
		return fmt.Errorf("update reserved count: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit reservation: %w", err)
	}
	return nil
}

// FindReservation retrieves a Reservation by its ID.
func (r *InventoryRepository) FindReservation(ctx context.Context, reservationID string) (*domain.Reservation, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, order_id, product_id, quantity, status, created_at FROM reservations WHERE id = $1`,
		reservationID,
	)
	res := &domain.Reservation{}
	if err := row.Scan(&res.ID, &res.OrderID, &res.ProductID, &res.Quantity, &res.Status, &res.CreatedAt); err != nil {
		return nil, fmt.Errorf("find reservation: %w", err)
	}
	return res, nil
}

// ReleaseReservation marks a reservation as released and increments the available stock.
func (r *InventoryRepository) ReleaseReservation(ctx context.Context, reservationID string) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var productID string
	var quantity int32
	err = tx.QueryRow(ctx,
		`UPDATE reservations SET status = 'released' WHERE id = $1 AND status = 'active'
		 RETURNING product_id, quantity`,
		reservationID,
	).Scan(&productID, &quantity)
	if err != nil {
		return fmt.Errorf("release reservation: %w", err)
	}

	_, err = tx.Exec(ctx,
		`UPDATE stock_items SET reserved = GREATEST(0, reserved - $1), updated_at = NOW() WHERE product_id = $2`,
		quantity, productID,
	)
	if err != nil {
		return fmt.Errorf("decrement reserved count: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit release: %w", err)
	}
	return nil
}
