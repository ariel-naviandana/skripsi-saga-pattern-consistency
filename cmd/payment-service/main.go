package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/ariel-naviandana/skripsi-saga-pattern-consistency/internal/common"
)

func main() {
	cfg := common.LoadConfig("payment")

	ctx := context.Background()
	pool, err := common.NewDB(ctx, common.ServicePayment)
	if err != nil {
		log.Fatalf("payment: connect db: %v", err)
	}
	defer pool.Close()

	if err := common.InitSchema(ctx, pool, common.PaymentSchema); err != nil {
		log.Fatalf("payment: init schema: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", common.HealthHandler("payment"))

	log.Printf("payment-service ready on %s", common.Addr(cfg.Port))
	if err := common.Serve(common.Addr(cfg.Port), mux); err != nil {
		log.Fatalf("payment: serve: %v", fmt.Errorf("serve: %w", err))
	}
}
