package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type runResult struct {
	Approach     string `json:"approach"`
	Count        int    `json:"count"`
	Committed    int    `json:"committed"`
	Compensated  int    `json:"compensated"`
	Inconsistent int    `json:"inconsistent"`
	NotFound     int    `json:"not_found"`
	AvgLatencyMS float64 `json:"avg_latency_ms"`
}

type agg struct {
	Scenario            string  `json:"scenario"`
	Approach            string  `json:"approach"`
	Runs                int     `json:"runs"`
	Transactions        int     `json:"transactions"`
	Committed           int     `json:"committed"`
	Compensated         int     `json:"compensated"`
	Inconsistent        int     `json:"inconsistent"`
	NotFound            int     `json:"not_found"`
	ConsistencyRate     float64 `json:"consistency_rate_pct"`
	CompSuccessRate     float64 `json:"compensating_success_rate_pct"`
	AvgRecoveryTimeMS   float64 `json:"avg_recovery_time_ms"`
	AvgLatencyMS        float64 `json:"avg_latency_ms"`
}

func main() {
	dir := flag.String("dir", "docs/runs", "runs directory")
	flag.Parse()

	var table []agg
	scenarios := []string{"S1", "S2", "S3", "S6", "S7"}
	approaches := []string{"choreography", "orchestration"}

	for _, s := range scenarios {
		for _, a := range approaches {
			dirPath := filepath.Join(*dir, s, a)
			files, err := filepath.Glob(filepath.Join(dirPath, "run-*.json"))
			if err != nil || len(files) == 0 {
				continue
			}
			var total runResult
			runs := 0
			for _, f := range files {
				data, err := os.ReadFile(f)
				if err != nil {
					continue
				}
				var rr runResult
				if err := json.Unmarshal(data, &rr); err != nil {
					continue
				}
				total.Count += rr.Count
				total.Committed += rr.Committed
				total.Compensated += rr.Compensated
				total.Inconsistent += rr.Inconsistent
				total.NotFound += rr.NotFound
				total.AvgLatencyMS += rr.AvgLatencyMS
				runs++
			}
			if runs == 0 {
				continue
			}
			txns := total.Count
			consistent := total.Committed + total.Compensated
			consistencyRate := 0.0
			if txns > 0 {
				consistencyRate = float64(consistent) / float64(txns) * 100
			}
			compAttempts := total.Compensated + total.Inconsistent + total.NotFound
			compSuccess := 0.0
			if compAttempts > 0 {
				compSuccess = float64(total.Compensated) / float64(compAttempts) * 100
			}
			table = append(table, agg{
				Scenario:          s,
				Approach:          a,
				Runs:              runs,
				Transactions:      txns,
				Committed:         total.Committed,
				Compensated:       total.Compensated,
				Inconsistent:      total.Inconsistent,
				NotFound:          total.NotFound,
				ConsistencyRate:   consistencyRate,
				CompSuccessRate:   compSuccess,
				AvgRecoveryTimeMS: total.AvgLatencyMS / float64(runs),
				AvgLatencyMS:      total.AvgLatencyMS / float64(runs),
			})
		}
	}

	sort.Slice(table, func(i, j int) bool {
		if table[i].Scenario != table[j].Scenario {
			return table[i].Scenario < table[j].Scenario
		}
		return table[i].Approach < table[j].Approach
	})

	// Table output
	fmt.Println("Scenario | Approach | Runs | Txns | Committed | Compensated | Inconsistent | NotFound | Consistency% | CompSuccess% | AvgLatency(ms)")
	fmt.Println("-------- | -------- | ---- | ---- | --------- | ----------- | ------------ | -------- | ------------ | ------------ | -------------")
	for _, a := range table {
		fmt.Printf("%s | %s | %d | %d | %d | %d | %d | %d | %.1f | %.1f | %.0f\n",
			a.Scenario, a.Approach, a.Runs, a.Transactions, a.Committed, a.Compensated,
			a.Inconsistent, a.NotFound, a.ConsistencyRate, a.CompSuccessRate, a.AvgLatencyMS)
	}

	// JSON summary
	out, _ := json.MarshalIndent(table, "", "  ")
	os.WriteFile(filepath.Join(*dir, "summary.json"), out, 0o644)
	fmt.Println("\nSummary written to", filepath.Join(*dir, "summary.json"))
	_ = strings.TrimSpace
}