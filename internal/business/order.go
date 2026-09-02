package business

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/ariel-naviandana/skripsi-saga-pattern-consistency/internal/common"
	"github.com/jackc/pgx/v5/pgxpool"
)

// OrderService owns the order database. It is shared by both approaches.
type OrderService struct {
	Pool  *pgxpool.Pool
	Fault *common.FaultConfig
}

// NewSagaID returns a random hex string used as the saga identifier.
func NewSagaID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Extremely unlikely; fall back to a time-based value rather than panic.
		return fmt.Sprintf("saga-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// CreateOrder persists a new order and logs the committed step.
// It returns the new order's id (as text) and the saga id.
func (s *OrderService) CreateOrder(ctx context.Context, sagaID, customerID, productID string, quantity, amount int) (string, error) {
	if s.Fault != nil {
		s.Fault.Delay()
		if s.Fault.ShouldFail("order", false) {
			_ = writeLog(ctx, s.Pool, sagaID, "order", "failed", "injected")
			return "", fmt.Errorf("business: injected order failure")
		}
	}
	var orderID int64
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO orders (saga_id, customer_id, product_id, quantity, amount, status)
		 VALUES ($1, $2, $3, $4, $5, 'committed') RETURNING order_id`,
		sagaID, customerID, productID, quantity, amount,
	).Scan(&orderID)
	if err != nil {
		return "", fmt.Errorf("business: insert order: %w", err)
	}
	if err := writeLog(ctx, s.Pool, sagaID, "order", "committed", fmt.Sprintf("order_id=%d", orderID)); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", orderID), nil
}

// CompensateOrder marks the order as compensated (rollback of CreateOrder).
func (s *OrderService) CompensateOrder(ctx context.Context, sagaID string) error {
	if s.Fault != nil {
		s.Fault.Delay()
		if s.Fault.ShouldFail("order", true) {
			_ = writeLog(ctx, s.Pool, sagaID, "order", "compensate_failed", "injected")
			return fmt.Errorf("business: injected order compensation failure")
		}
	}
	if _, err := s.Pool.Exec(ctx,
		`UPDATE orders SET status = 'compensated' WHERE saga_id = $1 AND status = 'committed'`,
		sagaID,
	); err != nil {
		return fmt.Errorf("business: compensate order: %w", err)
	}
	return writeLog(ctx, s.Pool, sagaID, "order", "compensated", "")
}