package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/ariel-naviandana/skripsi-saga-pattern-consistency/internal/common"
	"github.com/ariel-naviandana/skripsi-saga-pattern-consistency/internal/consistency"
)

type result struct {
	SagaID   string                   `json:"saga_id"`
	Outcome  consistency.SagaOutcome  `json:"outcome"`
	LatencyMS int64                   `json:"latency_ms"`
}

type summary struct {
	Approach     string    `json:"approach"`
	Count        int       `json:"count"`
	Committed    int       `json:"committed"`
	Compensated  int       `json:"compensated"`
	Inconsistent int       `json:"inconsistent"`
	NotFound     int       `json:"not_found"`
	TotalMS      int64     `json:"total_ms"`
	Throughput   float64   `json:"throughput_tps"`
	AvgLatencyMS float64   `json:"avg_latency_ms"`
	Results      []result  `json:"results,omitempty"`
}

// waitForOutcome polls the consistency checker until the saga reaches a
// stable final outcome. It stops early once committed/compensated is seen, or
// when an outcome repeats (stable), and gives up after the deadline.
func waitForOutcome(checker *consistency.Checker, sagaID string, deadline time.Duration) consistency.SagaOutcome {
	ctx := context.Background()
	start := time.Now()
	var last consistency.SagaOutcome
	stable := 0
	for {
		outcome, err := checker.Check(ctx, sagaID)
		if err != nil {
			return consistency.OutcomeInconsistent
		}
		if outcome == consistency.OutcomeCommitted || outcome == consistency.OutcomeCompensated {
			return outcome
		}
		if outcome == last {
			stable++
			if stable >= 2 {
				return outcome
			}
		} else {
			stable = 0
		}
		last = outcome
		if time.Since(start) > deadline {
			return outcome
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func main() {
	approach := flag.String("approach", "choreography", "choreography|orchestration")
	count := flag.Int("count", 10, "number of transactions")
	outFile := flag.String("out", "", "optional JSON output file")
	flag.Parse()

	baseURL := "http://localhost:8081/orders"
	if *approach == "orchestration" {
		baseURL = "http://localhost:8080/saga"
	}

	ctx := context.Background()
	checker, err := consistency.New(ctx, consistency.DefaultHostConfig())
	if err != nil {
		log.Fatalf("workload: consistency checker: %v", err)
	}
	defer checker.Close()

	start := time.Now()
	var results []result
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < *count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			body, _ := json.Marshal(common.OrderRequest{
				CustomerID: fmt.Sprintf("cust-%d", i),
				ProductID:  "product-1",
				Quantity:   1,
				Amount:     100000,
			})
			reqStart := time.Now()
			resp, err := http.Post(baseURL, "application/json", bytes.NewReader(body))
			if err != nil {
				log.Printf("workload: request %d failed: %v", i, err)
				return
			}
			var sagaID string
			var payload struct {
				SagaID string `json:"saga_id"`
			}
			_ = json.NewDecoder(resp.Body).Decode(&payload)
			resp.Body.Close()
			sagaID = payload.SagaID

			// Wait for the saga to settle. Choreography is asynchronous, so poll
			// until the outcome is stable (or a deadline passes).
			outcome := waitForOutcome(checker, sagaID, 10*time.Second)
			latency := time.Since(reqStart).Milliseconds()
			mu.Lock()
			results = append(results, result{SagaID: sagaID, Outcome: outcome, LatencyMS: latency})
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	elapsed := time.Since(start)
	sum := summary{
		Approach:   *approach,
		Count:      *count,
		TotalMS:    elapsed.Milliseconds(),
		Throughput: float64(*count) / elapsed.Seconds(),
		Results:    results,
	}
	for _, r := range results {
		switch r.Outcome {
		case consistency.OutcomeCommitted:
			sum.Committed++
		case consistency.OutcomeCompensated:
			sum.Compensated++
		case consistency.OutcomeInconsistent:
			sum.Inconsistent++
		default:
			sum.NotFound++
		}
		sum.AvgLatencyMS += float64(r.LatencyMS)
	}
	if len(results) > 0 {
		sum.AvgLatencyMS /= float64(len(results))
	}

	data, _ := json.MarshalIndent(sum, "", "  ")
	fmt.Println(string(data))
	if *outFile != "" {
		if err := os.WriteFile(*outFile, data, 0o644); err != nil {
			log.Fatalf("workload: write output: %v", err)
		}
	}
}