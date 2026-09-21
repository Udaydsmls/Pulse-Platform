package main

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB stores a row for every notification that was sent, so there is a record
// of what each customer was told and when.
type DB struct {
	pool *pgxpool.Pool
}

func (db *DB) CreateTable(ctx context.Context) error {
	_, err := db.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS notifications (
			id         SERIAL PRIMARY KEY,
			user_id    TEXT NOT NULL,
			event_type TEXT NOT NULL,
			channel    TEXT NOT NULL,
			message    TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL
		)
	`)
	return err
}

func (db *DB) Insert(ctx context.Context, n *Notification) error {
	_, err := db.pool.Exec(ctx, `
		INSERT INTO notifications (user_id, event_type, channel, message, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, n.UserID, n.EventType, n.Channel, n.Message, n.CreatedAt)
	return err
}
