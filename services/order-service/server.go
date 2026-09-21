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
	saga     *Saga
}

type createOrderRequest struct {
	UserID string      `json:"userId"`
	Email  string      `json:"email"`
	Items  []OrderItem `json:"items"`
}

type cancelOrderRequest struct {
	UserID string `json:"userId"`
}

type orderResponse struct {
	OrderID   string      `json:"orderId"`
	UserID    string      `json:"userId"`
	Status    string      `json:"status"`
	Items     []OrderItem `json:"items"`
	Total     float64     `json:"total"`
	CreatedAt string      `json:"createdAt"`
}

// CreateOrder saves a pending order and starts the saga by publishing
// order.created.
func (s *Server) CreateOrder(w http.ResponseWriter, r *http.Request) {
	var req createOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	order := NewOrder(uuid.NewString(), req.UserID, req.Email, req.Items)
	if err := order.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.db.Insert(r.Context(), order); err != nil {
		log.Printf("insert order: %v", err)
		writeError(w, http.StatusInternalServerError, "could not create order")
		return
	}

	// The order row is already saved, so a failed publish would leave it stuck
	// as pending. Report that instead of returning a success.
	err := s.producer.Publish(r.Context(), Event{
		Type:    "order.created",
		OrderID: order.ID,
		UserID:  order.UserID,
		Email:   order.Email,
		Items:   order.Items,
		Total:   order.Total,
	})
	if err != nil {
		log.Printf("publish order.created for %s: %v", order.ID, err)
		writeError(w, http.StatusInternalServerError, "order saved but could not be submitted")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"orderId": order.ID,
		"status":  order.Status,
	})
}

// GetOrder returns a single order.
func (s *Server) GetOrder(w http.ResponseWriter, r *http.Request) {
	order, err := s.db.FindByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "order not found")
		return
	}

	writeJSON(w, http.StatusOK, orderResponse{
		OrderID:   order.ID,
		UserID:    order.UserID,
		Status:    order.Status,
		Items:     order.Items,
		Total:     order.Total,
		CreatedAt: order.CreatedAt.Format(time.RFC3339),
	})
}

// CancelOrder cancels an order the customer asked to cancel. It reuses the
// saga's Cancel step, so the same order.cancelled event goes out and
// inventory-service releases the stock.
func (s *Server) CancelOrder(w http.ResponseWriter, r *http.Request) {
	var req cancelOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	order, err := s.db.FindByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "order not found")
		return
	}

	if order.UserID != req.UserID {
		writeError(w, http.StatusForbidden, "order belongs to another user")
		return
	}

	if !order.CanCancel() {
		writeError(w, http.StatusConflict, "order is already "+order.Status)
		return
	}

	if err := s.saga.Cancel(r.Context(), order.ID, "cancelled by customer"); err != nil {
		log.Printf("cancel order %s: %v", order.ID, err)
		writeError(w, http.StatusInternalServerError, "could not cancel order")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}
