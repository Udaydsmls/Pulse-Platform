package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DB runs the SQL queries for stock and reservations.
type DB struct {
	pool *pgxpool.Pool
}

func (db *DB) CreateTables(ctx context.Context) error {
	_, err := db.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS stock_items (
			product_id  TEXT PRIMARY KEY,
			stock_level INT NOT NULL DEFAULT 0,
			reserved    INT NOT NULL DEFAULT 0,
			updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TABLE IF NOT EXISTS reservations (
			id         TEXT PRIMARY KEY,
			order_id   TEXT NOT NULL,
			product_id TEXT NOT NULL,
			quantity   INT NOT NULL,
			status     TEXT NOT NULL DEFAULT 'active',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS reservations_order_id_idx ON reservations (order_id);
	`)
	return err
}

func (db *DB) FindStock(ctx context.Context, productID string) (*StockItem, error) {
	item := &StockItem{}
	err := db.pool.QueryRow(ctx, `
		SELECT product_id, stock_level, reserved, updated_at
		FROM stock_items WHERE product_id = $1
	`, productID).Scan(&item.ProductID, &item.StockLevel, &item.Reserved, &item.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return item, nil
}

// ReserveOrder reserves every item in an order, all or nothing. It runs in a
// single transaction and locks each product row (SELECT ... FOR UPDATE) so two
// orders cannot both take the last unit.
func (db *DB) ReserveOrder(ctx context.Context, orderID string, items []OrderItem) error {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return err
	}
	// No-op if the Commit at the end succeeded.
	defer tx.Rollback(ctx)

	for _, item := range items {
		var stockLevel, reserved int32
		err := tx.QueryRow(ctx, `
			SELECT stock_level, reserved FROM stock_items
			WHERE product_id = $1 FOR UPDATE
		`, item.ProductID).Scan(&stockLevel, &reserved)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("unknown product %s", item.ProductID)
			}
			return err
		}

		if available := stockLevel - reserved; available < item.Quantity {
			return fmt.Errorf("product %s: only %d left, wanted %d",
				item.ProductID, available, item.Quantity)
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO reservations (id, order_id, product_id, quantity)
			VALUES ($1, $2, $3, $4)
		`, uuid.NewString(), orderID, item.ProductID, item.Quantity)
		if err != nil {
			return err
		}

		_, err = tx.Exec(ctx, `
			UPDATE stock_items SET reserved = reserved + $1, updated_at = NOW()
			WHERE product_id = $2
		`, item.Quantity, item.ProductID)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// ReleaseOrder gives back everything reserved for an order. This is the saga's
// compensating action. It only touches rows still marked active, so calling it
// twice — or for an order that never reserved anything — does nothing the
// second time.
func (db *DB) ReleaseOrder(ctx context.Context, orderID string) error {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		UPDATE reservations SET status = 'released'
		WHERE order_id = $1 AND status = 'active'
		RETURNING product_id, quantity
	`, orderID)
	if err != nil {
		return err
	}

	// Read all the rows first; the connection can't run the updates below
	// while a query is still open on it.
	released := map[string]int32{}
	for rows.Next() {
		var productID string
		var quantity int32
		if err := rows.Scan(&productID, &quantity); err != nil {
			rows.Close()
			return err
		}
		released[productID] += quantity
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for productID, quantity := range released {
		_, err = tx.Exec(ctx, `
			UPDATE stock_items
			SET reserved = GREATEST(0, reserved - $1), updated_at = NOW()
			WHERE product_id = $2
		`, quantity, productID)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}
