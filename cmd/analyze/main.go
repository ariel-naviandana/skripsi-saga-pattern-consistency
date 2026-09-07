package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
)

type sagaResult struct {
	Outcome        string `json:"outcome"`
	LatencyMS      int64  `json:"latency_ms"`
	RecoveryTimeMS *int64 `json:"recovery_time_ms,omitempty"`
	InconsistencyMS *int64 `json:"inconsistency_ms,omitempty"`
}

type runResult struct {
	Approach     string       `json:"approach"`
	Count        int          `json:"count"`
	Committed    int          `json:"committed"`
	Compensated  int          `json:"compensated"`
	Inconsistent int          `json:"inconsistent"`
	NotFound     int          `json:"not_found"`
	AvgLatencyMS float64      `json:"avg_latency_ms"`
	Results      []sagaResult `json:"results,omitempty"`
}

type agg struct {
	Scenario          string  `json:"scenario"`
	Approach          string  `json:"approach"`
	Runs              int     `json:"runs"`
	Transactions      int     `json:"transactions"`
	Committed         int     `json:"committed"`
	Compensated       int     `json:"compensated"`
	Inconsistent      int     `json:"inconsistent"`
	NotFound          int     `json:"not_found"`
	Unrecorded        int     `json:"unrecorded"`
	ConsistencyRate   float64 `json:"consistency_rate_pct"`
	CompSuccessRate   float64 `json:"compensating_success_rate_pct"`
	RecoveryCount     int     `json:"recovery_count"`
	RecoveryAvgMS     float64 `json:"avg_recovery_time_ms"`
	RecoveryMinMS     float64 `json:"min_recovery_time_ms"`
	RecoveryMaxMS     float64 `json:"max_recovery_time_ms"`
	RecoveryStdMS     float64 `json:"std_recovery_time_ms"`
	InconsistencyCount int    `json:"inconsistency_count"`
	InconsistencyAvgMS float64 `json:"avg_inconsistency_ms"`
	InconsistencyMinMS float64 `json:"min_inconsistency_ms"`
	InconsistencyMaxMS float64 `json:"max_inconsistency_ms"`
	InconsistencyStdMS float64 `json:"std_inconsistency_ms"`
	LatencyAvgMS      float64 `json:"avg_latency_ms"`
	LatencyMinMS      float64 `json:"min_latency_ms"`
	LatencyMaxMS      float64 `json:"max_latency_ms"`
	LatencyStdMS      float64 `json:"std_latency_ms"`
}

func mean(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	var s float64
	for _, x := range v {
		s += x
	}
	return s / float64(len(v))
}

func minMax(v []float64) (float64, float64) {
	if len(v) == 0 {
		return 0, 0
	}
	mn, mx := v[0], v[0]
	for _, x := range v[1:] {
		if x < mn {
			mn = x
		}
		if x > mx {
			mx = x
		}
	}
	return mn, mx
}

// sampleStd returns the sample standard deviation.
func sampleStd(v []float64) float64 {
	if len(v) < 2 {
		return 0
	}
	m := mean(v)
	var s float64
	for _, x := range v {
		s += (x - m) * (x - m)
	}
	return math.Sqrt(s / float64(len(v)-1))
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
			var runConsistency, runCompSuccess []float64
			var latencies, recoveries, inconsistencies []float64
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
				runs++

				if rr.Count > 0 {
					runConsistency = append(runConsistency,
						float64(rr.Committed+rr.Compensated)/float64(rr.Count)*100)
				}
				attempts := rr.Compensated + rr.Inconsistent + rr.NotFound
				if attempts > 0 {
					runCompSuccess = append(runCompSuccess,
						float64(rr.Compensated)/float64(attempts)*100)
				}
				for _, r := range rr.Results {
					latencies = append(latencies, float64(r.LatencyMS))
					if r.RecoveryTimeMS != nil {
						recoveries = append(recoveries, float64(*r.RecoveryTimeMS))
					}
					if r.InconsistencyMS != nil {
						inconsistencies = append(inconsistencies, float64(*r.InconsistencyMS))
					}
				}
			}
			if runs == 0 {
				continue
			}
			recMin, recMax := minMax(recoveries)
			incMin, incMax := minMax(inconsistencies)
			latMin, latMax := minMax(latencies)
			table = append(table, agg{
				Scenario:        s,
				Approach:        a,
				Runs:            runs,
				Transactions:    total.Count,
				Committed:       total.Committed,
				Compensated:     total.Compensated,
				Inconsistent:    total.Inconsistent,
				NotFound:        total.NotFound,
				Unrecorded:      total.Count - total.Committed - total.Compensated - total.Inconsistent - total.NotFound,
				ConsistencyRate: mean(runConsistency),
				CompSuccessRate: mean(runCompSuccess),
				RecoveryCount:   len(recoveries),
				RecoveryAvgMS:   mean(recoveries),
				RecoveryMinMS:   recMin,
				RecoveryMaxMS:   recMax,
				RecoveryStdMS:   sampleStd(recoveries),
				InconsistencyCount: len(inconsistencies),
				InconsistencyAvgMS: mean(inconsistencies),
				InconsistencyMinMS: incMin,
				InconsistencyMaxMS: incMax,
				InconsistencyStdMS: sampleStd(inconsistencies),
				LatencyAvgMS:    mean(latencies),
				LatencyMinMS:    latMin,
				LatencyMaxMS:    latMax,
				LatencyStdMS:    sampleStd(latencies),
			})
		}
	}

	sort.Slice(table, func(i, j int) bool {
		if table[i].Scenario != table[j].Scenario {
			return table[i].Scenario < table[j].Scenario
		}
		return table[i].Approach < table[j].Approach
	})

	fmt.Println("Scenario | Approach | Runs | Txns | Committed | Compensated | Inconsistent | NotFound | Unrecorded | Consistency% | CompSuccess% | Rec# | AvgRec(ms) | MinRec | MaxRec | StdRec | Inc# | AvgInc(ms) | MinInc | MaxInc | StdInc | AvgLat(ms) | MinLat | MaxLat | StdLat")
	fmt.Println("-------- | -------- | ---- | ---- | --------- | ----------- | ------------ | -------- | ---------- | ------------ | ------------ | ---- | ---------- | ------ | ------ | ------ | ---- | ---------- | ------ | ------ | ------ | ---------- | ------ | ------ | ------")
	for _, a := range table {
		fmt.Printf("%s | %s | %d | %d | %d | %d | %d | %d | %d | %.1f | %.1f | %d | %.0f | %.0f | %.0f | %.0f | %d | %.0f | %.0f | %.0f | %.0f | %.0f | %.0f | %.0f | %.0f\n",
			a.Scenario, a.Approach, a.Runs, a.Transactions, a.Committed, a.Compensated,
			a.Inconsistent, a.NotFound, a.Unrecorded, a.ConsistencyRate, a.CompSuccessRate, a.RecoveryCount,
			a.RecoveryAvgMS, a.RecoveryMinMS, a.RecoveryMaxMS, a.RecoveryStdMS,
			a.InconsistencyCount, a.InconsistencyAvgMS, a.InconsistencyMinMS, a.InconsistencyMaxMS, a.InconsistencyStdMS,
			a.LatencyAvgMS, a.LatencyMinMS, a.LatencyMaxMS, a.LatencyStdMS)
	}

	out, _ := json.MarshalIndent(table, "", "  ")
	os.WriteFile(filepath.Join(*dir, "summary.json"), out, 0o644)
	fmt.Println("\nSummary written to", filepath.Join(*dir, "summary.json"))
}