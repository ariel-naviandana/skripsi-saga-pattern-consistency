package main

import (
	"log"
	"net/http"

	"github.com/ariel-naviandana/skripsi-saga-pattern-consistency/internal/common"
)

func main() {
	cfg := common.LoadConfig("orchestrator")

	mux := http.NewServeMux()
	mux.HandleFunc("/health", common.HealthHandler("orchestrator"))

	log.Printf("orchestrator ready on %s", common.Addr(cfg.Port))
	if err := common.Serve(common.Addr(cfg.Port), mux); err != nil {
		log.Fatalf("orchestrator: serve: %v", err)
	}
}
