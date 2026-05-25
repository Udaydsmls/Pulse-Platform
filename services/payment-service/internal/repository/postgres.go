package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pulse-platform/payment-service/internal/domain"
)

// PaymentRepository handles persistence for payment records.
type PaymentRepository struct {
	pool *pgxpool.Pool
}

// NewPaymentRepository creates a new PaymentRepository backed by the given connection pool.
func NewPaymentRepository(pool *pgxpool.Pool) *PaymentRepository {
	return &PaymentRepository{pool: pool}
}

// CreateTable creates the payments table if it does not exist.
func (r *PaymentRepository) CreateTable(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS payments (
			id             TEXT PRIMARY KEY,
			order_id       TEXT UNIQUE NOT NULL,
			user_id        TEXT NOT NULL,
			amount         DOUBLE PRECISION NOT NULL,
			currency       TEXT NOT NULL,
			status         TEXT NOT NULL DEFAULT 'pending',
			transaction_id TEXT NOT NULL DEFAULT '',
			created_at     TIMESTAMPTZ NOT NULL,
			updated_at     TIMESTAMPTZ NOT NULL
		);
	`)
	if err != nil {
		return fmt.Errorf("create payments table: %w", err)
	}
	return nil
}

// Create persists a new Payment record.
func (r *PaymentRepository) Create(ctx context.Context, p *domain.Payment) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO payments (id, order_id, user_id, amount, currency, status, transaction_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		p.ID, p.OrderID, p.UserID, p.Amount, p.Currency, string(p.Status), p.TransactionID, p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert payment: %w", err)
	}
	return nil
}

// FindByOrderID retrieves a Payment by its order ID.
func (r *PaymentRepository) FindByOrderID(ctx context.Context, orderID string) (*domain.Payment, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, order_id, user_id, amount, currency, status, transaction_id, created_at, updated_at
		 FROM payments WHERE order_id = $1`,
		orderID,
	)
	p := &domain.Payment{}
	var statusStr string
	if err := row.Scan(&p.ID, &p.OrderID, &p.UserID, &p.Amount, &p.Currency, &statusStr, &p.TransactionID, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return nil, fmt.Errorf("find payment by order id: %w", err)
	}
	p.Status = domain.PaymentStatus(statusStr)
	return p, nil
}

// UpdateStatus sets the status and transaction ID on an existing payment record.
func (r *PaymentRepository) UpdateStatus(ctx context.Context, paymentID, status, transactionID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE payments SET status = $1, transaction_id = $2, updated_at = NOW() WHERE id = $3`,
		status, transactionID, paymentID,
	)
	if err != nil {
		return fmt.Errorf("update payment status: %w", err)
	}
	return nil
}
