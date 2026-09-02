package common

import (
	"context"
	"fmt"

	"github.com/ariel-naviandana/skripsi-saga-pattern-consistency/pkg/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ServiceName string

const (
	ServiceOrder     ServiceName = "order"
	ServicePayment   ServiceName = "payment"
	ServiceInventory ServiceName = "inventory"
	ServiceShipping  ServiceName = "shipping"
	ServiceOrchestrator ServiceName = "orchestrator"
)

// ServiceDSN returns the default PostgreSQL DSN for a service name.
// These values match deployments/infrastructure.yml.
func ServiceDSN(name ServiceName) postgres.Config {
	switch name {
	case ServiceOrder:
		return postgres.Config{Host: "saga-order-db", Port: 5432, User: "order_user", Password: "order_pass", DBName: "order_db"}
	case ServicePayment:
		return postgres.Config{Host: "saga-payment-db", Port: 5432, User: "payment_user", Password: "payment_pass", DBName: "payment_db"}
	case ServiceInventory:
		return postgres.Config{Host: "saga-inventory-db", Port: 5432, User: "inventory_user", Password: "inventory_pass", DBName: "inventory_db"}
	case ServiceShipping:
		return postgres.Config{Host: "saga-shipping-db", Port: 5432, User: "shipping_user", Password: "shipping_pass", DBName: "shipping_db"}
	default:
		return postgres.Config{}
	}
}

func NewDB(ctx context.Context, name ServiceName) (*pgxpool.Pool, error) {
	cfg := ServiceDSN(name)
	pool, err := postgres.NewPool(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("common: db %s: %w", name, err)
	}
	return pool, nil
}
