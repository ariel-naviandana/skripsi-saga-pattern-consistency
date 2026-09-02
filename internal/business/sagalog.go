package business

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// writeLog inserts one row into saga_log for a service.
func writeLog(ctx context.Context, pool *pgxpool.Pool, sagaID, step, status, snapshot string) error {
	_, err := pool.Exec(ctx,
		`INSERT INTO saga_log (saga_id, step, status, snapshot) VALUES ($1, $2, $3, $4)`,
		sagaID, step, status, snapshot,
	)
	if err != nil {
		return fmt.Errorf("business: write saga_log: %w", err)
	}
	return nil
}