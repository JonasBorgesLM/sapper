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

func TestAggregateSumsStatusCountsAcrossRuns(t *testing.T) {
	snaps := []Snapshot{
		{StatusCounts: map[int]int{200: 90, 429: 10}},
		{StatusCounts: map[int]int{200: 80, 429: 20, 500: 1}},
	}
	agg := Aggregate(snaps)
	if agg.StatusCounts[200] != 170 || agg.StatusCounts[429] != 30 || agg.StatusCounts[500] != 1 {
		t.Errorf("StatusCounts = %v, want {200:170, 429:30, 500:1}", agg.StatusCounts)
	}
}

func TestKnee(t *testing.T) {
	// windows of 1s: w0 all 200 (100 req), w1 all 200 (100), w2 429 appears.
	windows := []WindowStat{
		{Start: 0, Total: 100, StatusCounts: map[int]int{200: 100}},
		{Start: time.Second, Total: 100, StatusCounts: map[int]int{200: 100}},
		{Start: 2 * time.Second, Total: 50, StatusCounts: map[int]int{429: 50}},
	}
	rps, at, ok := Knee(windows, time.Second, 429)
	if !ok {
		t.Fatal("Knee not found though 429 appears")
	}
	// cumulative 250 requests by end of window 2 (3s) => ~83.3 rps
	if rps < 80 || rps > 87 {
		t.Errorf("knee rps = %.1f, want ~83", rps)
	}
	if at != 3*time.Second {
		t.Errorf("knee at = %s, want 3s", at)
	}
	if _, _, ok := Knee(windows, time.Second, 503); ok {
		t.Error("Knee found a status that never appears")
	}
}
