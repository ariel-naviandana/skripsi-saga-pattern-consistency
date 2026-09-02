package common

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// InitSchema creates the saga_log table (shared by all services) plus any
// service-specific tables. It is idempotent (CREATE TABLE IF NOT EXISTS).
func InitSchema(ctx context.Context, pool *pgxpool.Pool, extraSQL ...string) error {
	const sagaLog = `
CREATE TABLE IF NOT EXISTS saga_log (
    id          BIGSERIAL PRIMARY KEY,
    saga_id     TEXT NOT NULL,
    step        TEXT NOT NULL,
    status      TEXT NOT NULL,
    snapshot    TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
`
	if _, err := pool.Exec(ctx, sagaLog); err != nil {
		return fmt.Errorf("common: create saga_log: %w", err)
	}
	for _, sql := range extraSQL {
		if _, err := pool.Exec(ctx, sql); err != nil {
			return fmt.Errorf("common: create table: %w", err)
		}
	}
	return nil
}

// OrderSchema is used by the order service.
const OrderSchema = `
CREATE TABLE IF NOT EXISTS orders (
    order_id    BIGSERIAL PRIMARY KEY,
    saga_id     TEXT NOT NULL,
    customer_id TEXT NOT NULL,
    product_id  TEXT NOT NULL,
    quantity    INT NOT NULL,
    amount      INT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'pending',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
`

// PaymentSchema is used by the payment service.
const PaymentSchema = `
CREATE TABLE IF NOT EXISTS payments (
    payment_id  BIGSERIAL PRIMARY KEY,
    saga_id     TEXT NOT NULL,
    order_id    TEXT NOT NULL,
    amount      INT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'pending',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
`

// InventorySchema is used by the inventory service.
const InventorySchema = `
CREATE TABLE IF NOT EXISTS inventory (
    id          BIGSERIAL PRIMARY KEY,
    saga_id     TEXT NOT NULL,
    product_id  TEXT NOT NULL,
    quantity    INT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'pending',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
`

// ShippingSchema is used by the shipping service.
const ShippingSchema = `
CREATE TABLE IF NOT EXISTS shipments (
    shipment_id BIGSERIAL PRIMARY KEY,
    saga_id     TEXT NOT NULL,
    order_id    TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'pending',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
`
