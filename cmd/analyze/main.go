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
	Needless       bool   `json:"needless_compensation,omitempty"`
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
	NeedlessCount      int     `json:"needless_compensation_count"`
	LatencyAvgMS      float64 `json:"avg_latency_ms"`
	LatencyMinMS      float64 `json:"min_latency_ms"`
	LatencyMaxMS      float64 `json:"max_latency_ms"`
	LatencyStdMS      float64 `json:"std_latency_ms"`
}

// sigResult is one Mann-Whitney U comparison (choreography vs orchestration)
// for a metric within a scenario, using per-run means as observations.
type sigResult struct {
	Scenario    string  `json:"scenario"`
	Metric      string  `json:"metric"`
	ChoreoMean  float64 `json:"choreo_mean_ms"`
	ChoreoStd   float64 `json:"choreo_std_ms"`
	OrchMean    float64 `json:"orch_mean_ms"`
	OrchStd     float64 `json:"orch_std_ms"`
	U           float64 `json:"u_statistic"`
	PValue      float64 `json:"p_value"`
	Significant bool    `json:"significant_alpha_005"`
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

// normCDF is the standard normal cumulative distribution function.
func normCDF(z float64) float64 {
	return 0.5 * (1 + math.Erf(z/math.Sqrt2))
}

// mannWhitneyU performs the two-sided Mann-Whitney U test using the normal
// approximation with continuity correction and tie correction (valid for
// n1, n2 >= 8, which holds for our 30-run samples). Returns the U statistic
// for sample x and the two-tailed p-value.
func mannWhitneyU(x, y []float64) (float64, float64) {
	n1, n2 := len(x), len(y)
	if n1 < 8 || n2 < 8 {
		return 0, math.NaN()
	}
	type item struct {
		val float64
		grp int
	}
	combined := make([]item, 0, n1+n2)
	for _, v := range x {
		combined = append(combined, item{v, 0})
	}
	for _, v := range y {
		combined = append(combined, item{v, 1})
	}
	sort.Slice(combined, func(i, j int) bool { return combined[i].val < combined[j].val })

	ranks := make([]float64, len(combined))
	for i := 0; i < len(combined); {
		j := i
		for j+1 < len(combined) && combined[j+1].val == combined[i].val {
			j++
		}
		avg := float64(i+j+2) / 2
		for k := i; k <= j; k++ {
			ranks[k] = avg
		}
		i = j + 1
	}
	var r1 float64
	for i, it := range combined {
		if it.grp == 0 {
			r1 += ranks[i]
		}
	}
	u1 := r1 - float64(n1*(n1+1))/2
	u := u1

	mu := float64(n1*n2) / 2
	n := float64(n1 + n2)

	var tieSum float64
	for i := 0; i < len(combined); {
		j := i
		for j+1 < len(combined) && combined[j+1].val == combined[i].val {
			j++
		}
		t := float64(j - i + 1)
		tieSum += t*t*t - t
		i = j + 1
	}
	sd := math.Sqrt(float64(n1*n2) / (n * (n - 1)) * ((n*n*n - n - tieSum) / 12))
	if sd == 0 {
		return u, math.NaN()
	}
	z := (u - mu + 0.5) / sd
	if u > mu {
		z = (u - mu - 0.5) / sd
	}
	p := 2 * (1 - normCDF(math.Abs(z)))
	if p > 1 {
		p = 1
	}
	return u, p
}

func main() {
	dir := flag.String("dir", "docs/runs", "runs directory")
	flag.Parse()

	var table []agg
	scenarios := []string{"S1", "S2", "S3", "S6", "S7", "S8", "S9", "S9s", "S2s", "S3s"}
	approaches := []string{"choreography", "orchestration"}
	metrics := []string{"latency", "inconsistency_window", "recovery_time"}

	// perRun[scenario][approach][metric] = per-run means (n = runs).
	perRun := map[string]map[string]map[string][]float64{}
	for _, s := range scenarios {
		perRun[s] = map[string]map[string][]float64{}
		for _, a := range approaches {
			perRun[s][a] = map[string][]float64{}
		}
	}

	for _, s := range scenarios {
		for _, a := range approaches {
			dirPath := filepath.Join(*dir, s, a)
			files, err := filepath.Glob(filepath.Join(dirPath, "run-*.json"))
			if err != nil || len(files) == 0 {
				continue
			}
			var total runResult
			runs := 0
			needlessCount := 0
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
				var runLat, runRec, runInc float64
				var nLat, nRec, nInc int
				for _, r := range rr.Results {
					latencies = append(latencies, float64(r.LatencyMS))
					runLat += float64(r.LatencyMS)
					nLat++
					if r.RecoveryTimeMS != nil {
						recoveries = append(recoveries, float64(*r.RecoveryTimeMS))
						runRec += float64(*r.RecoveryTimeMS)
						nRec++
					}
					if r.InconsistencyMS != nil {
						inconsistencies = append(inconsistencies, float64(*r.InconsistencyMS))
						runInc += float64(*r.InconsistencyMS)
						nInc++
					}
					if r.Needless {
						needlessCount++
					}
				}
				if nLat > 0 {
					perRun[s][a]["latency"] = append(perRun[s][a]["latency"], runLat/float64(nLat))
				}
				if nRec > 0 {
					perRun[s][a]["recovery_time"] = append(perRun[s][a]["recovery_time"], runRec/float64(nRec))
				}
				if nInc > 0 {
					perRun[s][a]["inconsistency_window"] = append(perRun[s][a]["inconsistency_window"], runInc/float64(nInc))
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
				NeedlessCount:     needlessCount,
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

	fmt.Println("Scenario | Approach | Runs | Txns | Committed | Compensated | Inconsistent | NotFound | Unrecorded | Needless | Consistency% | CompSuccess% | Rec# | AvgRec(ms) | MinRec | MaxRec | StdRec | Inc# | AvgInc(ms) | MinInc | MaxInc | StdInc | AvgLat(ms) | MinLat | MaxLat | StdLat")
	fmt.Println("-------- | -------- | ---- | ---- | --------- | ----------- | ------------ | -------- | ---------- | -------- | ------------ | ------------ | ---- | ---------- | ------ | ------ | ------ | ---- | ---------- | ------ | ------ | ------ | ---------- | ------ | ------ | ------")
	for _, a := range table {
		fmt.Printf("%s | %s | %d | %d | %d | %d | %d | %d | %d | %d | %.1f | %.1f | %d | %.0f | %.0f | %.0f | %.0f | %d | %.0f | %.0f | %.0f | %.0f | %.0f | %.0f | %.0f | %.0f\n",
			a.Scenario, a.Approach, a.Runs, a.Transactions, a.Committed, a.Compensated,
			a.Inconsistent, a.NotFound, a.Unrecorded, a.NeedlessCount, a.ConsistencyRate, a.CompSuccessRate, a.RecoveryCount,
			a.RecoveryAvgMS, a.RecoveryMinMS, a.RecoveryMaxMS, a.RecoveryStdMS,
			a.InconsistencyCount, a.InconsistencyAvgMS, a.InconsistencyMinMS, a.InconsistencyMaxMS, a.InconsistencyStdMS,
			a.LatencyAvgMS, a.LatencyMinMS, a.LatencyMaxMS, a.LatencyStdMS)
	}

	out, _ := json.MarshalIndent(table, "", "  ")
	os.WriteFile(filepath.Join(*dir, "summary.json"), out, 0o644)
	fmt.Println("\nSummary written to", filepath.Join(*dir, "summary.json"))

	// Mann-Whitney U comparisons (choreography vs orchestration) per metric.
	var sigs []sigResult
	for _, s := range scenarios {
		for _, m := range metrics {
			x := perRun[s]["choreography"][m]
			y := perRun[s]["orchestration"][m]
			if len(x) < 8 || len(y) < 8 {
				continue
			}
			u, p := mannWhitneyU(x, y)
			sigs = append(sigs, sigResult{
				Scenario:    s,
				Metric:      m,
				ChoreoMean:  mean(x),
				ChoreoStd:   sampleStd(x),
				OrchMean:    mean(y),
				OrchStd:     sampleStd(y),
				U:           u,
				PValue:      p,
				Significant: p < 0.05,
			})
		}
	}

	fmt.Println("\nMann-Whitney U (choreography vs orchestration, per-run means, alpha=0.05)")
	fmt.Println("Scenario | Metric | Choreo mean(ms) | Choreo std | Orch mean(ms) | Orch std | U | p-value | Significant")
	fmt.Println("-------- | ------ | --------------- | ---------- | ------------- | -------- | ------ | ------- | -----------")
	for _, s := range sigs {
		fmt.Printf("%s | %s | %.1f | %.1f | %.1f | %.1f | %.1f | %.4f | %v\n",
			s.Scenario, s.Metric, s.ChoreoMean, s.ChoreoStd, s.OrchMean, s.OrchStd, s.U, s.PValue, s.Significant)
	}

	sigOut, _ := json.MarshalIndent(sigs, "", "  ")
	os.WriteFile(filepath.Join(*dir, "significance.json"), sigOut, 0o644)
	fmt.Println("\nSignificance written to", filepath.Join(*dir, "significance.json"))
}