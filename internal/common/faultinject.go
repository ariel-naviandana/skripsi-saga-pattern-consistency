package common

import (
	"log"
	"sync/atomic"
	"time"
)

// FaultConfig reads fault injection parameters from environment variables.
// The middleware is non-intrusive: no service source changes between scenarios.
type FaultConfig struct {
	FailAtStep        string
	FailAtAttempt     int
	DelayMS           int
	FailOnCompensate  bool
	DropEvent         string
	DropResponseAtStep string
	SelectCompensate   bool
	attempt           atomic.Int32
	dropped           atomic.Bool
	droppedResp       atomic.Bool
}

// NewFaultConfig builds a FaultConfig from the process environment.
func NewFaultConfig() *FaultConfig {
	fc := &FaultConfig{
		FailAtStep:        EnvOr("FAIL_AT_STEP", ""),
		FailAtAttempt:     EnvIntOr("FAIL_AT_ATTEMPT", 0),
		DelayMS:           EnvIntOr("DELAY_MS", 0),
		FailOnCompensate:   EnvOr("FAIL_ON_COMPENSATE", "") == "true",
		DropEvent:         EnvOr("DROP_EVENT", ""),
		DropResponseAtStep: EnvOr("DROP_RESPONSE_AT_STEP", ""),
		SelectCompensate:   EnvOr("SELECTIVE_COMPENSATE", "") == "true",
	}
	return fc
}

// ResetAttempt zeroes the attempt counter (called between scenario runs).
func (f *FaultConfig) ResetAttempt() {
	f.attempt.Store(0)
	f.dropped.Store(false)
	f.droppedResp.Store(false)
}

// Delay sleeps for DelayMS if configured.
func (f *FaultConfig) Delay() {
	if f.DelayMS > 0 {
		time.Sleep(time.Duration(f.DelayMS) * time.Millisecond)
	}
}

// ShouldDropEvent reports whether the given event topic should be silently
// dropped (S8). It fires exactly once per process: the first publish to the
// configured topic is lost while the caller still reports success, simulating
// the dual-write / non-transactional queuing failure mode.
func (f *FaultConfig) ShouldDropEvent(topic string) bool {
	if f.DropEvent == "" || topic != f.DropEvent {
		return false
	}
	if f.dropped.Swap(true) {
		return false
	}
	log.Printf("faultinject: dropping event topic=%s", topic)
	return true
}

// ShouldDropResponse reports whether the HTTP response for the given forward
// step should be withheld (S9). It fires exactly once per process: the step
// commits its business transaction normally, but the response never reaches
// the orchestrator (the handler stalls past the orchestrator's timeout), so
// the orchestrator treats the step as failed and starts compensation — even
// though the operation actually succeeded (in-doubt / over-compensation).
func (f *FaultConfig) ShouldDropResponse(step string) bool {
	if f.DropResponseAtStep == "" || step != f.DropResponseAtStep {
		return false
	}
	if f.droppedResp.Swap(true) {
		return false
	}
	log.Printf("faultinject: dropping response step=%s", step)
	return true
}

// ShouldFail decides whether the current operation should fail. compensate
// distinguishes forward steps from compensating transactions (S6).
func (f *FaultConfig) ShouldFail(step string, compensate bool) bool {
	if f.FailAtStep == "" {
		return false
	}
	attempt := f.attempt.Add(1)
	if compensate {
		return f.FailOnCompensate
	}
	if step != f.FailAtStep {
		return false
	}
	if f.FailAtAttempt > 0 && attempt < int32(f.FailAtAttempt) {
		return false
	}
	log.Printf("faultinject: injecting failure at step=%s attempt=%d", step, attempt)
	return true
}