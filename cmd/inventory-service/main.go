package main

import (
	"context"
	"log"
	"net/http"

	"github.com/ariel-naviandana/skripsi-saga-pattern-consistency/internal/business"
	"github.com/ariel-naviandana/skripsi-saga-pattern-consistency/internal/choreography"
	"github.com/ariel-naviandana/skripsi-saga-pattern-consistency/internal/common"
	httph "github.com/ariel-naviandana/skripsi-saga-pattern-consistency/internal/http"
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

	biz := &business.InventoryService{Pool: pool, Fault: common.NewFaultConfig()}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", common.HealthHandler("inventory"))
	mux.HandleFunc("/fault/reset", common.FaultResetHandler(biz.Fault))

	switch cfg.Approach {
	case "orchestration":
		bh := &httph.BusinessHandler{Inventory: biz}
		mux.HandleFunc("/inventory", bh.InventoryReserve)
		mux.HandleFunc("/inventory/compensate", bh.InventoryCompensate)
	default: // choreography
		producer, err := choreography.NewProducer(cfg.KafkaBrokers, biz.Fault)
		if err != nil {
			log.Fatalf("inventory: kafka producer: %v", err)
		}
		defer producer.Close()

		choreo := &choreography.InventoryService{Biz: biz, Pub: producer, Brokers: cfg.KafkaBrokers}
		go func() {
			log.Printf("inventory: starting choreography consumers")
			if err := choreo.Run(ctx); err != nil {
				log.Printf("inventory: consumers stopped: %v", err)
			}
		}()
	}

	log.Printf("inventory-service ready (approach=%s) on %s", cfg.Approach, common.Addr(cfg.Port))
	if err := common.Serve(common.Addr(cfg.Port), mux); err != nil {
		log.Fatalf("inventory: serve: %v", err)
	}
}