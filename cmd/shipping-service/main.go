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

	biz := &business.ShippingService{Pool: pool, Fault: common.NewFaultConfig()}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", common.HealthHandler("shipping"))
	mux.HandleFunc("/fault/reset", common.FaultResetHandler(biz.Fault))

	switch cfg.Approach {
	case "orchestration":
		bh := &httph.BusinessHandler{Shipping: biz}
		mux.HandleFunc("/shipments", bh.ShippingSchedule)
		mux.HandleFunc("/shipments/compensate", bh.ShippingCompensate)
	default: // choreography
		producer, err := choreography.NewProducer(cfg.KafkaBrokers)
		if err != nil {
			log.Fatalf("shipping: kafka producer: %v", err)
		}
		defer producer.Close()

		choreo := &choreography.ShippingService{Biz: biz, Pub: producer, Brokers: cfg.KafkaBrokers}
		go func() {
			log.Printf("shipping: starting choreography consumers")
			if err := choreo.Run(ctx); err != nil {
				log.Printf("shipping: consumers stopped: %v", err)
			}
		}()
	}

	log.Printf("shipping-service ready (approach=%s) on %s", cfg.Approach, common.Addr(cfg.Port))
	if err := common.Serve(common.Addr(cfg.Port), mux); err != nil {
		log.Fatalf("shipping: serve: %v", err)
	}
}