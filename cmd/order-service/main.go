package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/ariel-naviandana/skripsi-saga-pattern-consistency/internal/common"
)

func main() {
	cfg := common.LoadConfig("order")

	ctx := context.Background()
	pool, err := common.NewDB(ctx, common.ServiceOrder)
	if err != nil {
		log.Fatalf("order: connect db: %v", err)
	}
	defer pool.Close()

	if err := common.InitSchema(ctx, pool, common.OrderSchema); err != nil {
		log.Fatalf("order: init schema: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", common.HealthHandler("order"))

	log.Printf("order-service ready on %s", common.Addr(cfg.Port))
	if err := common.Serve(common.Addr(cfg.Port), mux); err != nil {
		log.Fatalf("order: serve: %v", fmt.Errorf("serve: %w", err))
	}
}
