package main

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB holds the Postgres queries for users.
type DB struct {
	pool *pgxpool.Pool
}

func (db *DB) CreateTable(ctx context.Context) error {
	_, err := db.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS users (
			id            TEXT PRIMARY KEY,
			email         TEXT NOT NULL UNIQUE,
			name          TEXT NOT NULL,
			password_hash TEXT NOT NULL,
			created_at    TIMESTAMPTZ NOT NULL
		)
	`)
	return err
}

func (db *DB) Insert(ctx context.Context, u *User) error {
	_, err := db.pool.Exec(ctx, `
		INSERT INTO users (id, email, name, password_hash, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, u.ID, u.Email, u.Name, u.PasswordHash, u.CreatedAt)
	return err
}

func (db *DB) FindByEmail(ctx context.Context, email string) (*User, error) {
	return db.findOne(ctx, `WHERE email = $1`, email)
}

func (db *DB) FindByID(ctx context.Context, id string) (*User, error) {
	return db.findOne(ctx, `WHERE id = $1`, id)
}

// findOne runs a single-row lookup. The two finders differ only in their WHERE
// clause, so they share this helper.
func (db *DB) findOne(ctx context.Context, where string, args ...any) (*User, error) {
	query := `SELECT id, email, name, password_hash, created_at FROM users ` + where

	u := &User{}
	err := db.pool.QueryRow(ctx, query, args...).Scan(
		&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return u, nil
}
