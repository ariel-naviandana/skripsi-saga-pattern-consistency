package consistency

import (
	"context"
	"fmt"

	"github.com/ariel-naviandana/skripsi-saga-pattern-consistency/pkg/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Checker queries the four service databases to determine the consistency of
// a saga. The workload generator runs on the host, so it connects to the
// PostgreSQL containers via the published host ports.
type Checker struct {
	orderDB     *pgxpool.Pool
	paymentDB   *pgxpool.Pool
	inventoryDB *pgxpool.Pool
	shippingDB  *pgxpool.Pool
}

// HostConfig matches deployments/infrastructure.yml published ports.
type HostConfig struct {
	OrderHost     string `json:"order_host"`
	PaymentHost   string `json:"payment_host"`
	InventoryHost string `json:"inventory_host"`
	ShippingHost  string `json:"shipping_host"`
}

func DefaultHostConfig() HostConfig {
	return HostConfig{
		OrderHost:     "localhost:5431",
		PaymentHost:   "localhost:5432",
		InventoryHost: "localhost:5433",
		ShippingHost:  "localhost:5434",
	}
}

func New(ctx context.Context, cfg HostConfig) (*Checker, error) {
	mk := func(addr, user, pass, db string) (*pgxpool.Pool, error) {
		return postgres.NewPoolFromAddr(ctx, addr, user, pass, db)
	}
	orderDB, err := mk(cfg.OrderHost, "order_user", "order_pass", "order_db")
	if err != nil {
		return nil, fmt.Errorf("consistency: order db: %w", err)
	}
	paymentDB, err := mk(cfg.PaymentHost, "payment_user", "payment_pass", "payment_db")
	if err != nil {
		return nil, fmt.Errorf("consistency: payment db: %w", err)
	}
	inventoryDB, err := mk(cfg.InventoryHost, "inventory_user", "inventory_pass", "inventory_db")
	if err != nil {
		return nil, fmt.Errorf("consistency: inventory db: %w", err)
	}
	shippingDB, err := mk(cfg.ShippingHost, "shipping_user", "shipping_pass", "shipping_db")
	if err != nil {
		return nil, fmt.Errorf("consistency: shipping db: %w", err)
	}
	return &Checker{orderDB: orderDB, paymentDB: paymentDB, inventoryDB: inventoryDB, shippingDB: shippingDB}, nil
}

func (c *Checker) Close() {
	c.orderDB.Close()
	c.paymentDB.Close()
	c.inventoryDB.Close()
	c.shippingDB.Close()
}

func statusOf(ctx context.Context, pool *pgxpool.Pool, table string) (map[string]string, error) {
	rows, err := pool.Query(ctx, fmt.Sprintf(`SELECT saga_id, status FROM %s`, table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var sagaID, status string
		if err := rows.Scan(&sagaID, &status); err != nil {
			return nil, err
		}
		out[sagaID] = status
	}
	return out, nil
}

// sagaLogFlags returns, per saga_id, whether a forward failure and/or a
// compensation failure was recorded in saga_log.
func sagaLogFlags(ctx context.Context, pool *pgxpool.Pool) (failed map[string]bool, compFailed map[string]bool, err error) {
	failed = map[string]bool{}
	compFailed = map[string]bool{}
	rows, err := pool.Query(ctx, `SELECT saga_id, status FROM saga_log`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var sagaID, status string
		if err := rows.Scan(&sagaID, &status); err != nil {
			return nil, nil, err
		}
		switch status {
		case "failed":
			failed[sagaID] = true
		case "compensate_failed":
			compFailed[sagaID] = true
		}
	}
	return failed, compFailed, nil
}

// SagaOutcome classifies the final state of a saga across all services.
type SagaOutcome string

const (
	OutcomeCommitted    SagaOutcome = "committed"
	OutcomeCompensated  SagaOutcome = "compensated"
	OutcomeInconsistent SagaOutcome = "inconsistent"
	OutcomeNotFound     SagaOutcome = "not_found"
)

// Check returns the consistency outcome for one saga id.
//
// A service "participates" in a saga if it has a row in its business table.
// Services that never wrote a row (e.g. a step that failed before committing,
// or a step never reached) are ignored. The outcome is:
//   - committed: every participating service is committed
//   - compensated: every participating service is compensated
//   - inconsistent: a mix of committed/compensated, or a residual non-final state
func (c *Checker) Check(ctx context.Context, sagaID string) (SagaOutcome, error) {
	order, err := statusOf(ctx, c.orderDB, "orders")
	if err != nil {
		return "", fmt.Errorf("consistency: orders: %w", err)
	}
	payment, err := statusOf(ctx, c.paymentDB, "payments")
	if err != nil {
		return "", fmt.Errorf("consistency: payments: %w", err)
	}
	inventory, err := statusOf(ctx, c.inventoryDB, "inventory")
	if err != nil {
		return "", fmt.Errorf("consistency: inventory: %w", err)
	}
	shipping, err := statusOf(ctx, c.shippingDB, "shipments")
	if err != nil {
		return "", fmt.Errorf("consistency: shipments: %w", err)
	}

	statuses := []string{
		order[sagaID],
		payment[sagaID],
		inventory[sagaID],
		shipping[sagaID],
	}

	// Gather intent flags from saga_log across services.
	cf := false
	for _, pool := range []*pgxpool.Pool{c.orderDB, c.paymentDB, c.inventoryDB, c.shippingDB} {
		_, compFailed, err := sagaLogFlags(ctx, pool)
		if err != nil {
			return "", err
		}
		if compFailed[sagaID] {
			cf = true
		}
	}
	if cf {
		// A compensating transaction failed: the saga did not reach a clean end.
		return OutcomeInconsistent, nil
	}

	seen := false
	committed, compensated := 0, 0
	for _, st := range statuses {
		switch st {
		case "":
			// not participating
		case "committed":
			seen = true
			committed++
		case "compensated":
			seen = true
			compensated++
		default:
			return OutcomeInconsistent, nil
		}
	}
	if !seen {
		return OutcomeNotFound, nil
	}
	if committed > 0 && compensated == 0 {
		// A clean saga commits on all four services. A partial commit with no
		// failure marker means the saga was interrupted (e.g. orchestrator crash
		// or broker outage) and never finished.
		if committed == 4 {
			return OutcomeCommitted, nil
		}
		return OutcomeInconsistent, nil
	}
	if compensated > 0 && committed == 0 {
		return OutcomeCompensated, nil
	}
	return OutcomeInconsistent, nil
}