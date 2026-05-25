package domain

import (
	"context"
	"errors"
	"time"
)

// Provider represents the authentication provider for a user.
type Provider string

const (
	ProviderLocal  Provider = "local"
	ProviderGoogle Provider = "google"
	ProviderGithub Provider = "github"
)

// PasswordHasher defines the interface for hashing and comparing passwords.
type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) bool
}

// User is the core domain entity representing a platform user.
type User struct {
	ID           string
	Email        string
	Name         string
	PasswordHash string
	Provider     Provider
	ProviderID   string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// NewUser creates a new local-auth User with the given credentials.
func NewUser(email, name, passwordHash string) *User {
	now := time.Now().UTC()
	return &User{
		Email:        email,
		Name:         name,
		PasswordHash: passwordHash,
		Provider:     ProviderLocal,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

// NewOAuthUser creates a new User authenticated via an OAuth provider.
func NewOAuthUser(email, name string, provider Provider, providerID string) *User {
	now := time.Now().UTC()
	return &User{
		Email:      email,
		Name:       name,
		Provider:   provider,
		ProviderID: providerID,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// Validate checks that the User has all required fields populated.
func (u *User) Validate() error {
	if u.Email == "" {
		return errors.New("email is required")
	}
	if u.Name == "" {
		return errors.New("name is required")
	}
	if u.Provider == ProviderLocal && u.PasswordHash == "" {
		return errors.New("password hash is required for local users")
	}
	if u.Provider != ProviderLocal && u.ProviderID == "" {
		return errors.New("provider ID is required for OAuth users")
	}
	return nil
}

// MatchesPassword returns true if the given plaintext password matches the stored hash.
func (u *User) MatchesPassword(password string, hasher PasswordHasher) bool {
	return hasher.Compare(u.PasswordHash, password)
}

// UserRepository defines the persistence interface for User entities.
type UserRepository interface {
	Create(ctx context.Context, user *User) error
	FindByEmail(ctx context.Context, email string) (*User, error)
	FindByID(ctx context.Context, id string) (*User, error)
	FindByProviderID(ctx context.Context, provider Provider, providerID string) (*User, error)
}
