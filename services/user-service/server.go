package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// Server holds the dependencies the HTTP handlers need.
type Server struct {
	db       *DB
	producer *Producer
	secret   string
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	UserID string `json:"userId"`
	Token  string `json:"token"`
}

type userResponse struct {
	UserID    string `json:"userId"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
}

// Register creates an account and returns a JWT.
func (s *Server) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.Email == "" || req.Password == "" || req.Name == "" {
		writeError(w, http.StatusBadRequest, "email, password and name are required")
		return
	}

	hash, err := hashPassword(req.Password)
	if err != nil {
		log.Printf("hash password: %v", err)
		writeError(w, http.StatusInternalServerError, "could not create account")
		return
	}

	user := &User{
		ID:           uuid.NewString(),
		Email:        req.Email,
		Name:         req.Name,
		PasswordHash: hash,
		CreatedAt:    time.Now().UTC(),
	}

	if err := s.db.Insert(r.Context(), user); err != nil {
		// The email column is unique, so a duplicate signup lands here.
		writeError(w, http.StatusConflict, "email already registered")
		return
	}

	// The account exists either way, so a failed publish only costs the
	// customer a welcome email.
	if err := s.producer.PublishUserCreated(r.Context(), user.ID, user.Email); err != nil {
		log.Printf("publish user.created for %s: %v", user.ID, err)
	}

	writeJSON(w, http.StatusCreated, authResponse{UserID: user.ID, Token: s.issueToken(user)})
}

// Login checks a password and returns a JWT.
func (s *Server) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	user, err := s.db.FindByEmail(r.Context(), req.Email)
	// The same message for "no such user" and "wrong password" keeps the API
	// from confirming which emails are registered.
	if err != nil || !checkPassword(user.PasswordHash, req.Password) {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	writeJSON(w, http.StatusOK, authResponse{UserID: user.ID, Token: s.issueToken(user)})
}

// GetUser returns a user profile.
func (s *Server) GetUser(w http.ResponseWriter, r *http.Request) {
	user, err := s.db.FindByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	writeJSON(w, http.StatusOK, userResponse{
		UserID:    user.ID,
		Email:     user.Email,
		Name:      user.Name,
		CreatedAt: user.CreatedAt.Format(time.RFC3339),
	})
}

// issueToken signs a JWT for the user.
func (s *Server) issueToken(user *User) string {
	token, err := newToken(s.secret, user.ID, user.Email)
	if err != nil {
		log.Printf("sign token for %s: %v", user.ID, err)
	}
	return token
}
