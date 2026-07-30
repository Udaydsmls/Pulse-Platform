package main

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB holds the Postgres queries for payments.
type DB struct {
	pool *pgxpool.Pool
}

func (db *DB) CreateTable(ctx context.Context) error {
	_, err := db.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS payments (
			id             TEXT PRIMARY KEY,
			order_id       TEXT NOT NULL,
			user_id        TEXT NOT NULL,
			amount         DOUBLE PRECISION NOT NULL,
			currency       TEXT NOT NULL,
			status         TEXT NOT NULL,
			transaction_id TEXT NOT NULL DEFAULT '',
			created_at     TIMESTAMPTZ NOT NULL
		)
	`)
	return err
}

func (db *DB) Insert(ctx context.Context, p *Payment) error {
	_, err := db.pool.Exec(ctx, `
		INSERT INTO payments (id, order_id, user_id, amount, currency, status, transaction_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, p.ID, p.OrderID, p.UserID, p.Amount, p.Currency, p.Status, p.TransactionID, p.CreatedAt)
	return err
}

func (db *DB) UpdateStatus(ctx context.Context, id, status, transactionID string) error {
	_, err := db.pool.Exec(ctx, `
		UPDATE payments SET status = $1, transaction_id = $2 WHERE id = $3
	`, status, transactionID, id)
	return err
}
