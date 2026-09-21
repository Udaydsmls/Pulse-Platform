package main

import (
	"net/http"
	"strconv"
)

// Server holds the dependencies the HTTP handlers need. Reservations happen
// over Kafka as part of the saga; this is only the stock lookup.
type Server struct {
	db *DB
}

type stockResponse struct {
	Available  bool  `json:"available"`
	StockLevel int32 `json:"stockLevel"`
}

// CheckStock reports whether a product has enough units available.
func (s *Server) CheckStock(w http.ResponseWriter, r *http.Request) {
	quantity := int32(1)
	if raw := r.URL.Query().Get("quantity"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			writeError(w, http.StatusBadRequest, "quantity must be a positive number")
			return
		}
		quantity = int32(parsed)
	}

	item, err := s.db.FindStock(r.Context(), r.PathValue("productId"))
	if err != nil {
		writeError(w, http.StatusNotFound, "product not found")
		return
	}

	writeJSON(w, http.StatusOK, stockResponse{
		Available:  item.Available() >= quantity,
		StockLevel: item.Available(),
	})
}
