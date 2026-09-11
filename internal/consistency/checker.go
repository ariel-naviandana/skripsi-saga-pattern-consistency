package consistency

import (
	"context"
	"fmt"
	"time"

	"github.com/ariel-naviandana/skripsi-saga-pattern-consistency/pkg/postgres"
	"github.com/jackc/pgx/v5"
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
		OrderHost:     "127.0.0.1:5431",
		PaymentHost:   "127.0.0.1:5432",
		InventoryHost: "127.0.0.1:5433",
		ShippingHost:  "127.0.0.1:5434",
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

func statusOf(ctx context.Context, pool *pgxpool.Pool, table, sagaID string) (string, error) {
	var status string
	err := pool.QueryRow(ctx,
		fmt.Sprintf(`SELECT status FROM %s WHERE saga_id = $1`, table), sagaID).Scan(&status)
	if err == pgx.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return status, nil
}

// sagaLogFlags returns, for one saga, whether a forward failure and/or a
// compensation failure was recorded in saga_log.
func sagaLogFlags(ctx context.Context, pool *pgxpool.Pool, sagaID string) (failed, compFailed bool, err error) {
	rows, err := pool.Query(ctx,
		`SELECT status FROM saga_log WHERE saga_id = $1`, sagaID)
	if err != nil {
		return false, false, err
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		if err := rows.Scan(&status); err != nil {
			return false, false, err
		}
		switch status {
		case "failed":
			failed = true
		case "compensate_failed":
			compFailed = true
		}
	}
	return failed, compFailed, rows.Err()
}

// SagaOutcome classifies the final state of a saga across all services.
type SagaOutcome string

const (
	OutcomeCommitted    SagaOutcome = "committed"
	OutcomeCompensated  SagaOutcome = "compensated"
	OutcomeInconsistent SagaOutcome = "inconsistent"
	OutcomeNotFound     SagaOutcome = "not_found"
)

// Timeline returns the failure detection time, the final state time, and the
// first recorded time of a saga, based on saga_log timestamps across all four
// service databases.
//
// detection is the created_at of the first saga_log row with status "failed"
// (recorded by the fault injection middleware when the failure is triggered);
// final is the latest saga_log timestamp overall (the moment the last state
// change was confirmed); first is the earliest saga_log timestamp (when the
// saga started). Zero times are returned when the saga has no such row.
func (c *Checker) Timeline(ctx context.Context, sagaID string) (detection, final, first time.Time, err error) {
	pools := []*pgxpool.Pool{c.orderDB, c.paymentDB, c.inventoryDB, c.shippingDB}
	for _, pool := range pools {
		rows, err := pool.Query(ctx,
			`SELECT created_at, status FROM saga_log WHERE saga_id = $1 ORDER BY created_at`, sagaID)
		if err != nil {
			return time.Time{}, time.Time{}, time.Time{}, fmt.Errorf("consistency: timeline %s: %w", sagaID, err)
		}
		for rows.Next() {
			var ts time.Time
			var status string
			if err := rows.Scan(&ts, &status); err != nil {
				rows.Close()
				return time.Time{}, time.Time{}, time.Time{}, err
			}
			if status == "failed" && (detection.IsZero() || ts.Before(detection)) {
				detection = ts
			}
			if first.IsZero() || ts.Before(first) {
				first = ts
			}
			if final.IsZero() || ts.After(final) {
				final = ts
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return time.Time{}, time.Time{}, time.Time{}, err
		}
	}
	return detection, final, first, nil
}

// Sealed reports whether the saga recorded a compensating failure. A
// compensate_failed marker means the saga's fate is final: the compensation
// chain failed and no retry mechanism exists, so waiting for quiescence
// before classifying it is unnecessary.
func (c *Checker) Sealed(ctx context.Context, sagaID string) (bool, error) {
	for _, pool := range []*pgxpool.Pool{c.orderDB, c.paymentDB, c.inventoryDB, c.shippingDB} {
		_, compFailed, err := sagaLogFlags(ctx, pool, sagaID)
		if err != nil {
			return false, err
		}
		if compFailed {
			return true, nil
		}
	}
	return false, nil
}

// Check returns the consistency outcome for one saga id.
//
// A service "participates" in a saga if it has a row in its business table.
// Services that never wrote a row (e.g. a step that failed before committing,
// or a step never reached) are ignored. The outcome is:
//   - committed: every participating service is committed
//   - compensated: every participating service is compensated
//   - inconsistent: a mix of committed/compensated, or a residual non-final state
func (c *Checker) Check(ctx context.Context, sagaID string) (SagaOutcome, error) {
	order, err := statusOf(ctx, c.orderDB, "orders", sagaID)
	if err != nil {
		return "", fmt.Errorf("consistency: orders: %w", err)
	}
	payment, err := statusOf(ctx, c.paymentDB, "payments", sagaID)
	if err != nil {
		return "", fmt.Errorf("consistency: payments: %w", err)
	}
	inventory, err := statusOf(ctx, c.inventoryDB, "inventory", sagaID)
	if err != nil {
		return "", fmt.Errorf("consistency: inventory: %w", err)
	}
	shipping, err := statusOf(ctx, c.shippingDB, "shipments", sagaID)
	if err != nil {
		return "", fmt.Errorf("consistency: shipments: %w", err)
	}

	statuses := []string{
		order,
		payment,
		inventory,
		shipping,
	}

	// Gather intent flags from saga_log across services.
	cf := false
	for _, pool := range []*pgxpool.Pool{c.orderDB, c.paymentDB, c.inventoryDB, c.shippingDB} {
		_, compFailed, err := sagaLogFlags(ctx, pool, sagaID)
		if err != nil {
			return "", err
		}
		if compFailed {
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