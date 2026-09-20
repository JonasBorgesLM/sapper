package metrics

import (
	"testing"
	"time"
)

func TestResetDiscardsWarmupSamples(t *testing.T) {
	c := New()
	c.Record(ms, 200, nil)
	c.Record(ms, 200, nil)
	c.Reset() // called by the generator when the warm-up window ends (FR-07)
	if got := c.Snapshot().Total; got != 0 {
		t.Errorf("Total after Reset = %d, want 0 — warm-up samples must be discarded", got)
	}
}

func TestAggregateReportsMeanAndSpreadAcrossRuns(t *testing.T) {
	snaps := []Snapshot{
		{Total: 100, ErrorRate: 0.01, Latency: LatencyStats{P50: 10 * ms, P90: 50 * ms, P99: 100 * ms}},
		{Total: 100, ErrorRate: 0.02, Latency: LatencyStats{P50: 20 * ms, P90: 60 * ms, P99: 200 * ms}},
		{Total: 100, ErrorRate: 0.03, Latency: LatencyStats{P50: 30 * ms, P90: 70 * ms, P99: 300 * ms}},
	}
	agg := Aggregate(snaps)

	if agg.Runs != 3 {
		t.Errorf("Runs = %d, want 3", agg.Runs)
	}
	if agg.P99.Mean != 200*ms || agg.P99.Min != 100*ms || agg.P99.Max != 300*ms {
		t.Errorf("P99 = %+v, want mean 200ms / min 100ms / max 300ms", agg.P99)
	}
	// population stddev of {100,200,300}ms = sqrt(20000/3) ≈ 81.65ms
	if d := agg.P99.StdDev - 81650*time.Microsecond; d < -time.Millisecond || d > time.Millisecond {
		t.Errorf("P99.StdDev = %s, want ~81.65ms", agg.P99.StdDev)
	}
	if agg.ErrorRate.Mean < 0.0199 || agg.ErrorRate.Mean > 0.0201 {
		t.Errorf("ErrorRate.Mean = %v, want ~0.02", agg.ErrorRate.Mean)
	}
}

func TestAggregateEmptyIsZero(t *testing.T) {
	if agg := Aggregate(nil); agg.Runs != 0 {
		t.Errorf("Aggregate(nil).Runs = %d, want 0", agg.Runs)
	}
}
