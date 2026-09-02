package http

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/ariel-naviandana/skripsi-saga-pattern-consistency/internal/business"
	"github.com/ariel-naviandana/skripsi-saga-pattern-consistency/internal/common"
	"github.com/ariel-naviandana/skripsi-saga-pattern-consistency/internal/orchestration"
)

// BusinessHandler exposes the business operations over HTTP for the
// orchestration approach.
type BusinessHandler struct {
	Order     *business.OrderService
	Payment   *business.PaymentService
	Inventory *business.InventoryService
	Shipping  *business.ShippingService
}

// OrderProcess handles POST /orders/process.
func (h *BusinessHandler) OrderProcess(w http.ResponseWriter, r *http.Request) {
	var req orchestration.StepRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	orderID, err := h.Order.CreateOrder(r.Context(), req.SagaID, req.CustomerID, req.ProductID, req.Quantity, req.Amount)
	if err != nil {
		common.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	common.JSON(w, http.StatusOK, orchestration.StepResponse{OrderID: orderID, Status: "committed"})
}

// OrderCompensate handles POST /orders/compensate.
func (h *BusinessHandler) OrderCompensate(w http.ResponseWriter, r *http.Request) {
	compensate(w, r, func(sagaID string) error { return h.Order.CompensateOrder(r.Context(), sagaID) })
}

// PaymentProcess handles POST /payments.
func (h *BusinessHandler) PaymentProcess(w http.ResponseWriter, r *http.Request) {
	var req orchestration.StepRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if err := h.Payment.ProcessPayment(r.Context(), req.SagaID, req.OrderID, req.Amount); err != nil {
		common.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	common.JSON(w, http.StatusOK, orchestration.StepResponse{Status: "committed"})
}

// PaymentCompensate handles POST /payments/compensate.
func (h *BusinessHandler) PaymentCompensate(w http.ResponseWriter, r *http.Request) {
	compensate(w, r, func(sagaID string) error { return h.Payment.CompensatePayment(r.Context(), sagaID) })
}

// InventoryReserve handles POST /inventory.
func (h *BusinessHandler) InventoryReserve(w http.ResponseWriter, r *http.Request) {
	var req orchestration.StepRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if err := h.Inventory.ReserveInventory(r.Context(), req.SagaID, req.ProductID, req.Quantity); err != nil {
		common.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	common.JSON(w, http.StatusOK, orchestration.StepResponse{Status: "committed"})
}

// InventoryCompensate handles POST /inventory/compensate.
func (h *BusinessHandler) InventoryCompensate(w http.ResponseWriter, r *http.Request) {
	compensate(w, r, func(sagaID string) error { return h.Inventory.CompensateInventory(r.Context(), sagaID) })
}

// ShippingSchedule handles POST /shipments.
func (h *BusinessHandler) ShippingSchedule(w http.ResponseWriter, r *http.Request) {
	var req orchestration.StepRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if err := h.Shipping.ScheduleShipping(r.Context(), req.SagaID, req.OrderID); err != nil {
		common.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	common.JSON(w, http.StatusOK, orchestration.StepResponse{Status: "committed"})
}

// ShippingCompensate handles POST /shipments/compensate.
func (h *BusinessHandler) ShippingCompensate(w http.ResponseWriter, r *http.Request) {
	compensate(w, r, func(sagaID string) error { return h.Shipping.CompensateShipping(r.Context(), sagaID) })
}

func compensate(w http.ResponseWriter, r *http.Request, fn func(string) error) {
	var req orchestration.StepRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if err := fn(req.SagaID); err != nil {
		log.Printf("http: compensate %v", err)
		common.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	common.JSON(w, http.StatusOK, orchestration.StepResponse{Status: "compensated"})
}