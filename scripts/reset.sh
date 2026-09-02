#!/usr/bin/env bash
set -e

echo "Truncating databases..."
docker exec saga-order-db psql -U order_user -d order_db -c "TRUNCATE TABLE saga_log, orders RESTART IDENTITY CASCADE;" || true
docker exec saga-payment-db psql -U payment_user -d payment_db -c "TRUNCATE TABLE saga_log, payments RESTART IDENTITY CASCADE;" || true
docker exec saga-inventory-db psql -U inventory_user -d inventory_db -c "TRUNCATE TABLE saga_log, inventory RESTART IDENTITY CASCADE;" || true
docker exec saga-shipping-db psql -U shipping_user -d shipping_db -c "TRUNCATE TABLE saga_log, shipments RESTART IDENTITY CASCADE;" || true

echo "Flushing Redis (orchestrator state)..."
docker exec saga-redis redis-cli FLUSHALL

echo "Reset complete."
