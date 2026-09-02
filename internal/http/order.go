package http

import (
	"encoding/json"
	"net/http"

	"github.com/ariel-naviandana/skripsi-saga-pattern-consistency/internal/choreography"
	"github.com/ariel-naviandana/skripsi-saga-pattern-consistency/internal/common"
)

// OrderHandler serves the order entry point for the choreography approach.
type OrderHandler struct {
	Choreo *choreography.OrderService
}

// Create handles POST /orders: it persists the order and starts the saga.
func (h *OrderHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req common.OrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if req.Quantity <= 0 || req.Amount <= 0 {
		common.JSON(w, http.StatusBadRequest, map[string]string{"error": "quantity and amount must be positive"})
		return
	}
	sagaID, orderID, err := h.Choreo.CreateOrderAndPublish(r.Context(), req.CustomerID, req.ProductID, req.Quantity, req.Amount)
	if err != nil {
		common.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	common.JSON(w, http.StatusAccepted, map[string]string{
		"saga_id":  sagaID,
		"order_id": orderID,
		"status":   "pending",
	})
}