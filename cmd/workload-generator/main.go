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
	SagaID          string                   `json:"saga_id"`
	Outcome         consistency.SagaOutcome  `json:"outcome"`
	LatencyMS       int64                    `json:"latency_ms"`
	RecoveryTimeMS  *int64                   `json:"recovery_time_ms,omitempty"`
	InconsistencyMS *int64                   `json:"inconsistency_ms,omitempty"`
	Needless        bool                     `json:"needless_compensation,omitempty"`
	ErrorMsg        string                   `json:"error,omitempty"`
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
	RecoveryCount     int     `json:"recovery_count"`
	RecoveryTotalMS   int64   `json:"recovery_total_ms"`
	RecoveryMinMS     int64   `json:"recovery_min_ms"`
	RecoveryMaxMS     int64   `json:"recovery_max_ms"`
	AvgRecoveryTimeMS float64 `json:"avg_recovery_time_ms"`
	InconsistencyCount     int     `json:"inconsistency_count"`
	InconsistencyTotalMS   int64   `json:"inconsistency_total_ms"`
	InconsistencyMinMS     int64   `json:"inconsistency_min_ms"`
	InconsistencyMaxMS     int64   `json:"inconsistency_max_ms"`
	AvgInconsistencyMS     float64 `json:"avg_inconsistency_ms"`
	NeedlessCount          int     `json:"needless_compensation_count"`
	Results      []result  `json:"results,omitempty"`
}

// waitForOutcome polls the consistency checker until the saga reaches a
// stable final outcome. A saga is considered final when it is committed or
// compensated, or when its saga_log stops advancing (quiescent) for a while.
// The quiescence window must exceed the largest inter-step gap: under load
// (S7) the Kafka chain is serialized, so a later saga can legitimately wait
// many seconds between steps (observed up to ~7s for 500 concurrent orders).
// 10s separates "slow but progressing" from "stuck".
func waitForOutcome(checker *consistency.Checker, sagaID string, deadline time.Duration) consistency.SagaOutcome {
	const quiescence = 10 * time.Second
	ctx := context.Background()
	start := time.Now()
	var lastOutcome consistency.SagaOutcome
	var lastProgress time.Time
	stable := 0
	for {
		outcome, err := checker.Check(ctx, sagaID)
		if err != nil {
			return consistency.OutcomeInconsistent
		}
		if _, fin, _, terr := checker.Timeline(ctx, sagaID); terr == nil && !fin.IsZero() {
			if lastProgress.IsZero() || fin.After(lastProgress) {
				lastProgress = fin
				stable = 0
			}
		}
		if outcome == consistency.OutcomeCommitted || outcome == consistency.OutcomeCompensated {
			return outcome
		}
		// A compensate_failed marker seals the saga: nothing will retry, so the
		// outcome (inconsistent) is final without waiting for quiescence.
		if sealed, serr := checker.Sealed(ctx, sagaID); serr == nil && sealed {
			return outcome
		}
		if !lastProgress.IsZero() && time.Since(lastProgress) > quiescence {
			if outcome == lastOutcome {
				stable++
				if stable >= 2 {
					return outcome
				}
			} else {
				stable = 0
			}
		}
		lastOutcome = outcome
		if time.Since(start) > deadline {
			return outcome
		}
		time.Sleep(1500 * time.Millisecond)
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
				errMsg := err.Error()
				log.Printf("workload: request %d failed: %v", i, err)
				mu.Lock()
				results = append(results, result{SagaID: "", Outcome: "not_found", LatencyMS: time.Since(reqStart).Milliseconds(), ErrorMsg: errMsg})
				mu.Unlock()
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
			// until the outcome is stable (or a deadline passes). The 60s
			// deadline accommodates high concurrency (S7) where some sagas take
			// longer than the old 10s window to finish the event chain.
			outcome := waitForOutcome(checker, sagaID, 60*time.Second)

			// Recovery time (proposal 3.5.3): from failure detection to final
			// consistent state. Only meaningful for sagas that hit a failure and
			// recovered (compensated). Inconsistent sagas never recovered, and
			// cleanly committed sagas had no failure to recover from.
			var recoveryMS *int64
			var inconsistencyMS *int64
			needless := false
			if outcome == consistency.OutcomeCompensated {
				// A compensated saga without any "failed" marker in saga_log was
				// cancelled even though no business step ever failed (S9: the
				// orchestrator over-compensated because the step's response was
				// lost). Mark it as a needless compensation.
				if det, fin, _, terr := checker.Timeline(ctx, sagaID); terr == nil && !fin.IsZero() {
					if !det.IsZero() {
						ms := fin.Sub(det).Milliseconds()
						recoveryMS = &ms
					} else {
						needless = true
					}
				}
			}
			// Temporary inconsistency window (proposal 3.7): from the first
			// saga_log write until the final state — every saga passes through a
			// transient inconsistent state before settling.
			if det, fin, first, terr := checker.Timeline(ctx, sagaID); terr == nil && !first.IsZero() && !fin.IsZero() {
				ms := fin.Sub(first).Milliseconds()
				inconsistencyMS = &ms
				_ = det
			}

			latency := time.Since(reqStart).Milliseconds()
			mu.Lock()
			results = append(results, result{
				SagaID:          sagaID,
				Outcome:         outcome,
				LatencyMS:       latency,
				RecoveryTimeMS:  recoveryMS,
				InconsistencyMS: inconsistencyMS,
				Needless:        needless,
			})
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

	var recCount int
	var recTotal, recMin, recMax int64
	for _, r := range results {
		if r.RecoveryTimeMS == nil {
			continue
		}
		if recCount == 0 {
			recMin, recMax = *r.RecoveryTimeMS, *r.RecoveryTimeMS
		} else {
			if *r.RecoveryTimeMS < recMin {
				recMin = *r.RecoveryTimeMS
			}
			if *r.RecoveryTimeMS > recMax {
				recMax = *r.RecoveryTimeMS
			}
		}
		recTotal += *r.RecoveryTimeMS
		recCount++
	}
	sum.RecoveryCount = recCount
	sum.RecoveryTotalMS = recTotal
	sum.RecoveryMinMS = recMin
	sum.RecoveryMaxMS = recMax
	if recCount > 0 {
		sum.AvgRecoveryTimeMS = float64(recTotal) / float64(recCount)
	}

	var incCount int
	var incTotal, incMin, incMax int64
	for _, r := range results {
		if r.InconsistencyMS == nil {
			continue
		}
		if incCount == 0 {
			incMin, incMax = *r.InconsistencyMS, *r.InconsistencyMS
		} else {
			if *r.InconsistencyMS < incMin {
				incMin = *r.InconsistencyMS
			}
			if *r.InconsistencyMS > incMax {
				incMax = *r.InconsistencyMS
			}
		}
		incTotal += *r.InconsistencyMS
		incCount++
	}
	sum.InconsistencyCount = incCount
	sum.InconsistencyTotalMS = incTotal
	sum.InconsistencyMinMS = incMin
	sum.InconsistencyMaxMS = incMax
	if incCount > 0 {
		sum.AvgInconsistencyMS = float64(incTotal) / float64(incCount)
	}

	for _, r := range results {
		if r.Needless {
			sum.NeedlessCount++
		}
	}

	data, _ := json.MarshalIndent(sum, "", "  ")
	fmt.Println(string(data))
	if *outFile != "" {
		if err := os.WriteFile(*outFile, data, 0o644); err != nil {
			log.Fatalf("workload: write output: %v", err)
		}
	}
}