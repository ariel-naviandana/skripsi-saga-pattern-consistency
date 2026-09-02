package business

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PaymentService owns the payment database.
type PaymentService struct {
	Pool *pgxpool.Pool
}

// ProcessPayment records a successful payment for the saga.
func (s *PaymentService) ProcessPayment(ctx context.Context, sagaID, orderID string, amount int) error {
	if _, err := s.Pool.Exec(ctx,
		`INSERT INTO payments (saga_id, order_id, amount, status) VALUES ($1, $2, $3, 'committed')`,
		sagaID, orderID, amount,
	); err != nil {
		return fmt.Errorf("business: insert payment: %w", err)
	}
	return writeLog(ctx, s.Pool, sagaID, "payment", "committed", fmt.Sprintf("order_id=%s amount=%d", orderID, amount))
}

// CompensatePayment marks the payment as compensated.
func (s *PaymentService) CompensatePayment(ctx context.Context, sagaID string) error {
	if _, err := s.Pool.Exec(ctx,
		`UPDATE payments SET status = 'compensated' WHERE saga_id = $1 AND status = 'committed'`,
		sagaID,
	); err != nil {
		return fmt.Errorf("business: compensate payment: %w", err)
	}
	return writeLog(ctx, s.Pool, sagaID, "payment", "compensated", "")
}