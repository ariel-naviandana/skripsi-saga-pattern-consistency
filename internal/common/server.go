package common

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// JSON writes a JSON response.
func JSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("common: encode json: %v", err)
	}
}

// HealthHandler returns service liveness.
func HealthHandler(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		JSON(w, http.StatusOK, map[string]string{
			"service": name,
			"status":  "ok",
			"time":    time.Now().UTC().Format(time.RFC3339),
		})
	}
}

// Serve starts an HTTP server and blocks until ctx is cancelled.
func Serve(addr string, handler http.Handler) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("common: listening on %s", addr)
	return srv.ListenAndServe()
}

func Addr(port int) string {
	return fmt.Sprintf(":%d", port)
}

// FaultResetHandler resets the fault attempt counter (called between runs).
func FaultResetHandler(fc *FaultConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if fc != nil {
			fc.ResetAttempt()
		}
		JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
