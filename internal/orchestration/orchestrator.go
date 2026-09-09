package orchestration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/ariel-naviandana/skripsi-saga-pattern-consistency/internal/business"
	"github.com/ariel-naviandana/skripsi-saga-pattern-consistency/internal/common"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// Orchestrator coordinates a saga sequentially via HTTP request-reply and
// keeps the saga state in Redis.
type Orchestrator struct {
	OrderURL       string
	PaymentURL     string
	InventoryURL   string
	ShippingURL    string
	Redis          *redis.Client
	client         *http.Client
	Fault          *common.FaultConfig
	orderDB        *pgxpool.Pool
	paymentDB      *pgxpool.Pool
	inventoryDB    *pgxpool.Pool
	shippingDB     *pgxpool.Pool
}

func New(order, payment, inventory, shipping string, rdb *redis.Client, fault *common.FaultConfig) *Orchestrator {
	return &Orchestrator{
		OrderURL:     order,
		PaymentURL:   payment,
		InventoryURL: inventory,
		ShippingURL:  shipping,
		Redis:        rdb,
		client:       &http.Client{Timeout: 10 * time.Second},
		Fault:        fault,
	}
}

// SetDBs wires the four service databases so the orchestrator can query
// per-saga status when SELECTIVE_COMPENSATE is enabled.
func (o *Orchestrator) SetDBs(order, payment, inventory, shipping *pgxpool.Pool) {
	o.orderDB = order
	o.paymentDB = payment
	o.inventoryDB = inventory
	o.shippingDB = shipping
}

// stepCommitted reports whether the given step recorded a "committed" row for
// the saga. Returns true (compensate) when status is unknown (defensive default
// for backwards-compatible behavior when SELECTIVE_COMPENSATE is off).
func (o *Orchestrator) stepCommitted(ctx context.Context, pool *pgxpool.Pool, table, sagaID string) bool {
	if pool == nil {
		log.Printf("orchestrator: stepCommitted(%s) pool nil — defaulting to compensate", table)
		return true
	}
	var status string
	err := pool.QueryRow(ctx,
		fmt.Sprintf(`SELECT status FROM %s WHERE saga_id = $1`, table), sagaID).Scan(&status)
	if err == pgx.ErrNoRows {
		return false
	}
	if err != nil {
		log.Printf("orchestrator: stepCommitted(%s, %s) db error (fallback to compensate): %v", table, sagaID, err)
		return true
	}
	return status == "committed"
}

func redisKey(sagaID string) string { return "saga:" + sagaID }

func (o *Orchestrator) saveState(ctx context.Context, sagaID, state string) {
	if err := o.Redis.Set(ctx, redisKey(sagaID), state, 24*time.Hour).Err(); err != nil {
		log.Printf("orchestrator: save state %s=%s: %v", sagaID, state, err)
	}
}

// call sends a JSON request to url and decodes the StepResponse.
func (o *Orchestrator) call(ctx context.Context, method, url string, body any) (StepResponse, error) {
	var resp StepResponse
	data, err := json.Marshal(body)
	if err != nil {
		return resp, err
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(data))
	if err != nil {
		return resp, err
	}
	req.Header.Set("Content-Type", "application/json")
	httpResp, err := o.client.Do(req)
	if err != nil {
		return resp, err
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != http.StatusOK {
		return resp, fmt.Errorf("service returned %s", httpResp.Status)
	}
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		return resp, err
	}
	return resp, nil
}

// Start executes the full saga for one order.
func (o *Orchestrator) Start(ctx context.Context, req StartRequest) (StartResponse, error) {
	sagaID := business.NewSagaID()
	step := StepRequest{
		SagaID:     sagaID,
		CustomerID: req.CustomerID,
		ProductID:  req.ProductID,
		Quantity:   req.Quantity,
		Amount:     req.Amount,
	}
	o.saveState(ctx, sagaID, "order")

	orderResp, err := o.call(ctx, http.MethodPost, o.OrderURL+"/orders/process", step)
	if err != nil {
		return StartResponse{SagaID: sagaID, Status: "compensated"}, o.fail(ctx, sagaID, fmt.Errorf("order: %w", err))
	}
	step.OrderID = orderResp.OrderID
	o.saveState(ctx, sagaID, "payment")

	if _, err := o.call(ctx, http.MethodPost, o.PaymentURL+"/payments", step); err != nil {
		return StartResponse{SagaID: sagaID, Status: "compensated"}, o.fail(ctx, sagaID, fmt.Errorf("payment: %w", err))
	}
	o.saveState(ctx, sagaID, "inventory")

	if _, err := o.call(ctx, http.MethodPost, o.InventoryURL+"/inventory", step); err != nil {
		return StartResponse{SagaID: sagaID, Status: "compensated"}, o.fail(ctx, sagaID, fmt.Errorf("inventory: %w", err))
	}
	o.saveState(ctx, sagaID, "shipping")

	if _, err := o.call(ctx, http.MethodPost, o.ShippingURL+"/shipments", step); err != nil {
		return StartResponse{SagaID: sagaID, Status: "compensated"}, o.fail(ctx, sagaID, fmt.Errorf("shipping: %w", err))
	}
	o.saveState(ctx, sagaID, "committed")

	return StartResponse{SagaID: sagaID, Status: "committed"}, nil
}

// fail runs compensating transactions in reverse order and returns the error.
// When SELECTIVE_COMPENSATE is enabled, it only compensates services that
// actually recorded a "committed" row — testing the counterfactual of the
// call-all design: orphaned commits may appear if a service committed but was
// not selected for compensation.
func (o *Orchestrator) fail(ctx context.Context, sagaID string, cause error) error {
	o.saveState(ctx, sagaID, "compensating")

	type compStep struct {
		url     string
		wasCommitted bool
	}
	steps := []compStep{
		{o.ShippingURL + "/shipments/compensate", o.stepCommitted(ctx, o.shippingDB, "shipments", sagaID)},
		{o.InventoryURL + "/inventory/compensate", o.stepCommitted(ctx, o.inventoryDB, "inventory", sagaID)},
		{o.PaymentURL + "/payments/compensate", o.stepCommitted(ctx, o.paymentDB, "payments", sagaID)},
		{o.OrderURL + "/orders/compensate", o.stepCommitted(ctx, o.orderDB, "orders", sagaID)},
	}
	for _, c := range steps {
		if o.Fault != nil && o.Fault.SelectCompensate && !c.wasCommitted {
			log.Printf("orchestrator: skip compensate (not committed) %s", c.url)
			continue
		}
		body := StepRequest{SagaID: sagaID}
		if _, err := o.call(ctx, http.MethodPost, c.url, body); err != nil {
			log.Printf("orchestrator: compensate %s: %v", c.url, err)
		}
	}
	o.saveState(ctx, sagaID, "compensated")
	return cause
}

// State returns the current saga state from Redis.
func (o *Orchestrator) State(ctx context.Context, sagaID string) (string, error) {
	return o.Redis.Get(ctx, redisKey(sagaID)).Result()
}