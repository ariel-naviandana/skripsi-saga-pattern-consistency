package business

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ShippingService owns the shipping database.
type ShippingService struct {
	Pool *pgxpool.Pool
}

// ScheduleShipping records a shipping schedule for the saga.
func (s *ShippingService) ScheduleShipping(ctx context.Context, sagaID, orderID string) error {
	if _, err := s.Pool.Exec(ctx,
		`INSERT INTO shipments (saga_id, order_id, status) VALUES ($1, $2, 'committed')`,
		sagaID, orderID,
	); err != nil {
		return fmt.Errorf("business: insert shipment: %w", err)
	}
	return writeLog(ctx, s.Pool, sagaID, "shipping", "committed", fmt.Sprintf("order_id=%s", orderID))
}

// CompensateShipping marks the shipment as compensated.
func (s *ShippingService) CompensateShipping(ctx context.Context, sagaID string) error {
	if _, err := s.Pool.Exec(ctx,
		`UPDATE shipments SET status = 'compensated' WHERE saga_id = $1 AND status = 'committed'`,
		sagaID,
	); err != nil {
		return fmt.Errorf("business: compensate shipping: %w", err)
	}
	return writeLog(ctx, s.Pool, sagaID, "shipping", "compensated", "")
}