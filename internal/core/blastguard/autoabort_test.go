package blastguard

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/JonasBorgesLM/sapper/internal/core/model"
)

func TestAutoAbortLimitsExceeded(t *testing.T) {
	limits := AutoAbortLimits{ErrorRateOver: 0.5, P99Over: 2 * time.Second}

	if _, ok := limits.Exceeded(0.2, time.Second); ok {
		t.Error("under both ceilings reported as exceeded")
	}
	if reason, ok := limits.Exceeded(0.9, time.Second); !ok || !strings.Contains(reason, "error rate") {
		t.Errorf("error rate over ceiling: ok=%v reason=%q", ok, reason)
	}
	if reason, ok := limits.Exceeded(0.1, 3*time.Second); !ok || !strings.Contains(reason, "p99") {
		t.Errorf("p99 over ceiling: ok=%v reason=%q", ok, reason)
	}
}

func TestAutoAbortLimitsZeroMeansDisabled(t *testing.T) {
	var disabled AutoAbortLimits // both zero
	if _, ok := disabled.Exceeded(1.0, time.Hour); ok {
		t.Error("zero limits must never trip — a zero ceiling means the signal is off")
	}
}

func TestWatchAutoAbortStopsGuardWhenExceeded(t *testing.T) {
	g, err := New(model.TierLab, testCaps)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	// A sample that is always over the error-rate ceiling.
	sample := func() (float64, time.Duration) { return 0.9, 0 }
	done := make(chan struct{})
	go func() {
		WatchAutoAbort(context.Background(), g, sample, AutoAbortLimits{ErrorRateOver: 0.5}, time.Millisecond)
		close(done)
	}()
	<-done // WatchAutoAbort returns once it has stopped the guard

	if !g.Stopped() {
		t.Fatal("guard not stopped after the ceiling was crossed (SR-05)")
	}
	if !strings.Contains(g.Reason(), "error rate") {
		t.Errorf("Reason() = %q, want it to name the error rate", g.Reason())
	}
}
