package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/ariel-naviandana/skripsi-saga-pattern-consistency/internal/common"
	"github.com/ariel-naviandana/skripsi-saga-pattern-consistency/internal/orchestration"
	"github.com/ariel-naviandana/skripsi-saga-pattern-consistency/pkg/redis"
)

func main() {
	cfg := common.LoadConfig("orchestrator")

	ctx := context.Background()
	rdb, err := redis.NewClient(ctx, cfg.RedisAddr)
	if err != nil {
		log.Fatalf("orchestrator: redis: %v", err)
	}
	defer rdb.Close()

	orch := orchestration.New(
		"http://order-service:8081",
		"http://payment-service:8082",
		"http://inventory-service:8083",
		"http://shipping-service:8084",
		rdb,
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", common.HealthHandler("orchestrator"))
	mux.HandleFunc("/saga", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			common.JSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})
			return
		}
		var req orchestration.StartRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		resp, err := orch.Start(r.Context(), req)
		if err != nil {
			log.Printf("orchestrator: saga failed: %v", err)
		}
		common.JSON(w, http.StatusOK, resp)
	})
	mux.HandleFunc("/state/", func(w http.ResponseWriter, r *http.Request) {
		sagaID := r.URL.Path[len("/state/"):]
		if sagaID == "" {
			common.JSON(w, http.StatusBadRequest, map[string]string{"error": "missing saga_id"})
			return
		}
		state, err := orch.State(r.Context(), sagaID)
		if err != nil {
			common.JSON(w, http.StatusNotFound, map[string]string{"error": "saga not found"})
			return
		}
		common.JSON(w, http.StatusOK, map[string]string{"saga_id": sagaID, "state": state})
	})

	log.Printf("orchestrator ready on %s", common.Addr(cfg.Port))
	if err := common.Serve(common.Addr(cfg.Port), mux); err != nil {
		log.Fatalf("orchestrator: serve: %v", err)
	}
}