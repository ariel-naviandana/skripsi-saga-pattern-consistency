package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/ariel-naviandana/skripsi-saga-pattern-consistency/internal/common"
)

func main() {
	cfg := common.LoadConfig("inventory")

	ctx := context.Background()
	pool, err := common.NewDB(ctx, common.ServiceInventory)
	if err != nil {
		log.Fatalf("inventory: connect db: %v", err)
	}
	defer pool.Close()

	if err := common.InitSchema(ctx, pool, common.InventorySchema); err != nil {
		log.Fatalf("inventory: init schema: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", common.HealthHandler("inventory"))

	log.Printf("inventory-service ready on %s", common.Addr(cfg.Port))
	if err := common.Serve(common.Addr(cfg.Port), mux); err != nil {
		log.Fatalf("inventory: serve: %v", fmt.Errorf("serve: %w", err))
	}
}
