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

	biz := &business.OrderService{Pool: pool, Fault: common.NewFaultConfig()}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", common.HealthHandler("order"))
	mux.HandleFunc("/fault/reset", common.FaultResetHandler(biz.Fault))

	switch cfg.Approach {
	case "orchestration":
		bh := &httph.BusinessHandler{Order: biz}
		mux.HandleFunc("/orders/process", bh.OrderProcess)
		mux.HandleFunc("/orders/compensate", bh.OrderCompensate)
	default: // choreography
		producer, err := choreography.NewProducer(cfg.KafkaBrokers, biz.Fault)
		if err != nil {
			log.Fatalf("order: kafka producer: %v", err)
		}
		defer producer.Close()

		choreo := &choreography.OrderService{Biz: biz, Pub: producer, Brokers: cfg.KafkaBrokers}
		mux.HandleFunc("/orders", (&httph.OrderHandler{Choreo: choreo}).Create)

		go func() {
			log.Printf("order: starting choreography consumers")
			if err := choreo.Run(ctx); err != nil {
				log.Printf("order: consumers stopped: %v", err)
			}
		}()
	}

	log.Printf("order-service ready (approach=%s) on %s", cfg.Approach, common.Addr(cfg.Port))
	if err := common.Serve(common.Addr(cfg.Port), mux); err != nil {
		log.Fatalf("order: serve: %v", err)
	}
}