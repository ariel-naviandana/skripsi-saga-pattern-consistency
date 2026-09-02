package common

import (
	"log"
	"sync/atomic"
	"time"
)

// FaultConfig reads fault injection parameters from environment variables.
// The middleware is non-intrusive: no service source changes between scenarios.
type FaultConfig struct {
	FailAtStep      string
	FailAtAttempt   int
	DelayMS         int
	FailOnCompensate bool
	attempt         atomic.Int32
}

// NewFaultConfig builds a FaultConfig from the process environment.
func NewFaultConfig() *FaultConfig {
	fc := &FaultConfig{
		FailAtStep:      EnvOr("FAIL_AT_STEP", ""),
		FailAtAttempt:   EnvIntOr("FAIL_AT_ATTEMPT", 0),
		DelayMS:         EnvIntOr("DELAY_MS", 0),
		FailOnCompensate: EnvOr("FAIL_ON_COMPENSATE", "") == "true",
	}
	return fc
}

// ResetAttempt zeroes the attempt counter (called between scenario runs).
func (f *FaultConfig) ResetAttempt() {
	f.attempt.Store(0)
}

// Delay sleeps for DelayMS if configured.
func (f *FaultConfig) Delay() {
	if f.DelayMS > 0 {
		time.Sleep(time.Duration(f.DelayMS) * time.Millisecond)
	}
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