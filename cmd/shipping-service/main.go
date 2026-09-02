package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/ariel-naviandana/skripsi-saga-pattern-consistency/internal/common"
)

func main() {
	cfg := common.LoadConfig("shipping")

	ctx := context.Background()
	pool, err := common.NewDB(ctx, common.ServiceShipping)
	if err != nil {
		log.Fatalf("shipping: connect db: %v", err)
	}
	defer pool.Close()

	if err := common.InitSchema(ctx, pool, common.ShippingSchema); err != nil {
		log.Fatalf("shipping: init schema: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", common.HealthHandler("shipping"))

	log.Printf("shipping-service ready on %s", common.Addr(cfg.Port))
	if err := common.Serve(common.Addr(cfg.Port), mux); err != nil {
		log.Fatalf("shipping: serve: %v", fmt.Errorf("serve: %w", err))
	}
}
