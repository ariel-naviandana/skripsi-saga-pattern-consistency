package business

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// InventoryService owns the inventory database.
type InventoryService struct {
	Pool *pgxpool.Pool
}

// ReserveInventory records a stock reservation for the saga.
func (s *InventoryService) ReserveInventory(ctx context.Context, sagaID, productID string, quantity int) error {
	if _, err := s.Pool.Exec(ctx,
		`INSERT INTO inventory (saga_id, product_id, quantity, status) VALUES ($1, $2, $3, 'committed')`,
		sagaID, productID, quantity,
	); err != nil {
		return fmt.Errorf("business: insert inventory: %w", err)
	}
	return writeLog(ctx, s.Pool, sagaID, "inventory", "committed", fmt.Sprintf("product_id=%s quantity=%d", productID, quantity))
}

// CompensateInventory marks the reservation as compensated.
func (s *InventoryService) CompensateInventory(ctx context.Context, sagaID string) error {
	if _, err := s.Pool.Exec(ctx,
		`UPDATE inventory SET status = 'compensated' WHERE saga_id = $1 AND status = 'committed'`,
		sagaID,
	); err != nil {
		return fmt.Errorf("business: compensate inventory: %w", err)
	}
	return writeLog(ctx, s.Pool, sagaID, "inventory", "compensated", "")
}