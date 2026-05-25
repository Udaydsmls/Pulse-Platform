package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pulse-platform/user-service/internal/domain"
)

// UserRepository implements domain.UserRepository backed by PostgreSQL.
type UserRepository struct {
	pool *pgxpool.Pool
}

// NewUserRepository creates a new UserRepository using the given connection pool.
func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

// CreateTable ensures the users table exists in the database.
func (r *UserRepository) CreateTable(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS users (
			id            TEXT PRIMARY KEY,
			email         TEXT NOT NULL UNIQUE,
			name          TEXT NOT NULL,
			password_hash TEXT NOT NULL DEFAULT '',
			provider      TEXT NOT NULL DEFAULT 'local',
			provider_id   TEXT NOT NULL DEFAULT '',
			created_at    TIMESTAMPTZ NOT NULL,
			updated_at    TIMESTAMPTZ NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("create users table: %w", err)
	}
	return nil
}

// Create inserts a new user record into the database.
func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO users (id, email, name, password_hash, provider, provider_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`,
		user.ID,
		user.Email,
		user.Name,
		user.PasswordHash,
		string(user.Provider),
		user.ProviderID,
		user.CreatedAt,
		user.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// FindByEmail retrieves a user by their email address.
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	user := &domain.User{}
	var provider string
	err := r.pool.QueryRow(ctx, `
		SELECT id, email, name, password_hash, provider, provider_id, created_at, updated_at
		FROM users WHERE email = $1
	`, email).Scan(
		&user.ID,
		&user.Email,
		&user.Name,
		&user.PasswordHash,
		&provider,
		&user.ProviderID,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user not found: %w", err)
		}
		return nil, fmt.Errorf("find user by email: %w", err)
	}
	user.Provider = domain.Provider(provider)
	return user, nil
}

// FindByID retrieves a user by their unique ID.
func (r *UserRepository) FindByID(ctx context.Context, id string) (*domain.User, error) {
	user := &domain.User{}
	var provider string
	err := r.pool.QueryRow(ctx, `
		SELECT id, email, name, password_hash, provider, provider_id, created_at, updated_at
		FROM users WHERE id = $1
	`, id).Scan(
		&user.ID,
		&user.Email,
		&user.Name,
		&user.PasswordHash,
		&provider,
		&user.ProviderID,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user not found: %w", err)
		}
		return nil, fmt.Errorf("find user by id: %w", err)
	}
	user.Provider = domain.Provider(provider)
	return user, nil
}

// FindByProviderID retrieves a user by their OAuth provider and provider-specific ID.
func (r *UserRepository) FindByProviderID(ctx context.Context, provider domain.Provider, providerID string) (*domain.User, error) {
	user := &domain.User{}
	var providerStr string
	err := r.pool.QueryRow(ctx, `
		SELECT id, email, name, password_hash, provider, provider_id, created_at, updated_at
		FROM users WHERE provider = $1 AND provider_id = $2
	`, string(provider), providerID).Scan(
		&user.ID,
		&user.Email,
		&user.Name,
		&user.PasswordHash,
		&providerStr,
		&user.ProviderID,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user not found: %w", err)
		}
		return nil, fmt.Errorf("find user by provider id: %w", err)
	}
	user.Provider = domain.Provider(providerStr)
	return user, nil
}
