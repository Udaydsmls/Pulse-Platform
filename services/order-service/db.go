package main

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB holds the Postgres queries for orders.
type DB struct {
	pool *pgxpool.Pool
}

func (db *DB) CreateTable(ctx context.Context) error {
	_, err := db.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS orders (
			id         TEXT PRIMARY KEY,
			user_id    TEXT NOT NULL,
			email      TEXT NOT NULL DEFAULT '',
			status     TEXT NOT NULL,
			items      JSONB NOT NULL DEFAULT '[]',
			total      DOUBLE PRECISION NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL
		)
	`)
	return err
}

func (db *DB) Insert(ctx context.Context, o *Order) error {
	items, err := json.Marshal(o.Items)
	if err != nil {
		return err
	}

	_, err = db.pool.Exec(ctx, `
		INSERT INTO orders (id, user_id, email, status, items, total, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, o.ID, o.UserID, o.Email, o.Status, items, o.Total, o.CreatedAt)
	return err
}

func (db *DB) FindByID(ctx context.Context, id string) (*Order, error) {
	o := &Order{}
	var items []byte

	err := db.pool.QueryRow(ctx, `
		SELECT id, user_id, email, status, items, total, created_at
		FROM orders WHERE id = $1
	`, id).Scan(&o.ID, &o.UserID, &o.Email, &o.Status, &items, &o.Total, &o.CreatedAt)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(items, &o.Items); err != nil {
		return nil, err
	}
	return o, nil
}

func (db *DB) UpdateStatus(ctx context.Context, id, status string) error {
	_, err := db.pool.Exec(ctx, `UPDATE orders SET status = $1 WHERE id = $2`, status, id)
	return err
}
