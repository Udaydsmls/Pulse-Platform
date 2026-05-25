package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pulse-platform/order-service/internal/domain"
)

// OrderRepository implements persistence for Order entities using PostgreSQL.
type OrderRepository struct {
	pool *pgxpool.Pool
}

// NewOrderRepository creates a new OrderRepository using the given connection pool.
func NewOrderRepository(pool *pgxpool.Pool) *OrderRepository {
	return &OrderRepository{pool: pool}
}

// CreateTable ensures the orders table exists in the database.
func (r *OrderRepository) CreateTable(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS orders (
			id         TEXT PRIMARY KEY,
			user_id    TEXT NOT NULL,
			tenant_id  TEXT NOT NULL,
			status     TEXT NOT NULL,
			items      JSONB NOT NULL DEFAULT '[]',
			total      DOUBLE PRECISION NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("create orders table: %w", err)
	}
	return nil
}

// Create inserts a new order record into the database.
func (r *OrderRepository) Create(ctx context.Context, order *domain.Order) error {
	itemsJSON, err := json.Marshal(order.Items)
	if err != nil {
		return fmt.Errorf("marshal order items: %w", err)
	}

	_, err = r.pool.Exec(ctx, `
		INSERT INTO orders (id, user_id, tenant_id, status, items, total, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`,
		order.ID,
		order.UserID,
		order.TenantID,
		string(order.Status),
		itemsJSON,
		order.Total,
		order.CreatedAt,
		order.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create order: %w", err)
	}
	return nil
}

// FindByID retrieves an order by its unique ID.
func (r *OrderRepository) FindByID(ctx context.Context, id string) (*domain.Order, error) {
	order := &domain.Order{}
	var statusStr string
	var itemsJSON []byte

	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, tenant_id, status, items, total, created_at, updated_at
		FROM orders WHERE id = $1
	`, id).Scan(
		&order.ID,
		&order.UserID,
		&order.TenantID,
		&statusStr,
		&itemsJSON,
		&order.Total,
		&order.CreatedAt,
		&order.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("order not found: %w", err)
		}
		return nil, fmt.Errorf("find order by id: %w", err)
	}

	order.Status = domain.OrderStatus(statusStr)
	if err := json.Unmarshal(itemsJSON, &order.Items); err != nil {
		return nil, fmt.Errorf("unmarshal order items: %w", err)
	}
	return order, nil
}

// FindByUserID retrieves all orders belonging to a user.
func (r *OrderRepository) FindByUserID(ctx context.Context, userID string) ([]*domain.Order, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, tenant_id, status, items, total, created_at, updated_at
		FROM orders WHERE user_id = $1 ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("query orders by user id: %w", err)
	}
	defer rows.Close()

	var orders []*domain.Order
	for rows.Next() {
		order := &domain.Order{}
		var statusStr string
		var itemsJSON []byte

		if err := rows.Scan(
			&order.ID,
			&order.UserID,
			&order.TenantID,
			&statusStr,
			&itemsJSON,
			&order.Total,
			&order.CreatedAt,
			&order.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan order row: %w", err)
		}

		order.Status = domain.OrderStatus(statusStr)
		if err := json.Unmarshal(itemsJSON, &order.Items); err != nil {
			return nil, fmt.Errorf("unmarshal order items: %w", err)
		}
		orders = append(orders, order)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate order rows: %w", err)
	}
	return orders, nil
}

// UpdateStatus changes the status of an existing order.
func (r *OrderRepository) UpdateStatus(ctx context.Context, orderID string, status domain.OrderStatus) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE orders SET status = $1, updated_at = NOW() WHERE id = $2
	`, string(status), orderID)
	if err != nil {
		return fmt.Errorf("update order status: %w", err)
	}
	return nil
}
