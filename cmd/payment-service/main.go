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

	biz := &business.PaymentService{Pool: pool}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", common.HealthHandler("payment"))

	switch cfg.Approach {
	case "orchestration":
		bh := &httph.BusinessHandler{Payment: biz}
		mux.HandleFunc("/payments", bh.PaymentProcess)
		mux.HandleFunc("/payments/compensate", bh.PaymentCompensate)
	default: // choreography
		producer, err := choreography.NewProducer(cfg.KafkaBrokers)
		if err != nil {
			log.Fatalf("payment: kafka producer: %v", err)
		}
		defer producer.Close()

		choreo := &choreography.PaymentService{Biz: biz, Pub: producer, Brokers: cfg.KafkaBrokers}
		go func() {
			log.Printf("payment: starting choreography consumers")
			if err := choreo.Run(ctx); err != nil {
				log.Printf("payment: consumers stopped: %v", err)
			}
		}()
	}

	log.Printf("payment-service ready (approach=%s) on %s", cfg.Approach, common.Addr(cfg.Port))
	if err := common.Serve(common.Addr(cfg.Port), mux); err != nil {
		log.Fatalf("payment: serve: %v", err)
	}
}