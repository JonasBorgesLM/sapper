package assert

import (
	"testing"
	"time"

	"github.com/JonasBorgesLM/sapper/internal/core/metrics"
	"github.com/JonasBorgesLM/sapper/internal/core/model"
	"github.com/JonasBorgesLM/sapper/internal/core/scenario"
)

func dur(d time.Duration) *scenario.Duration { sd := scenario.Duration(d); return &sd }
func rate(f float64) *float64                { return &f }

func aggWith(p99 time.Duration, errRate float64) metrics.Aggregation {
	return metrics.Aggregation{
		Runs:      3,
		P99:       metrics.DurationStat{Mean: p99, StdDev: 10 * time.Millisecond},
		ErrorRate: metrics.RateStat{Mean: errRate},
	}
}

func TestEvaluatePassesWhenUnderThresholds(t *testing.T) {
	v := Evaluate(aggWith(150*time.Millisecond, 0.005), scenario.SLOs{
		P99Under:       dur(200 * time.Millisecond),
		ErrorRateUnder: rate(0.01),
	})
	if !v.Passed {
		t.Errorf("Verdict.Passed = false, want true; results = %+v", v.Results)
	}
	if len(v.Results) != 2 {
		t.Errorf("len(Results) = %d, want 2 (one per declared SLO)", len(v.Results))
	}
}

func TestEvaluateFailsWhenP99Over(t *testing.T) {
	v := Evaluate(aggWith(250*time.Millisecond, 0.0), scenario.SLOs{P99Under: dur(200 * time.Millisecond)})
	if v.Passed {
		t.Error("Verdict.Passed = true, want false when p99 exceeds the SLO")
	}
	if len(v.Results) != 1 || v.Results[0].Passed {
		t.Errorf("Results = %+v, want a single failed p99 result", v.Results)
	}
}

func TestEvaluateFailsWhenErrorRateOver(t *testing.T) {
	v := Evaluate(aggWith(10*time.Millisecond, 0.2), scenario.SLOs{ErrorRateUnder: rate(0.01)})
	if v.Passed {
		t.Error("Verdict.Passed = true, want false when the error rate exceeds the SLO")
	}
}

func TestEvaluateStatusSeen(t *testing.T) {
	agg := metrics.Aggregation{Runs: 1, StatusCounts: map[int]int{200: 100, 429: 5}}

	seen := 429
	if v := Evaluate(agg, scenario.SLOs{StatusSeen: &seen}); !v.Passed {
		t.Errorf("status_seen 429 failed though 429 appeared: %+v", v.Results)
	}
	notSeen := 503
	if v := Evaluate(agg, scenario.SLOs{StatusSeen: &notSeen}); v.Passed {
		t.Error("status_seen 503 passed though 503 never appeared")
	}
}

func TestRecovery(t *testing.T) {
	before := metrics.Snapshot{Latency: metrics.LatencyStats{P99: 100 * time.Millisecond}}

	ok := metrics.Snapshot{Latency: metrics.LatencyStats{P99: 120 * time.Millisecond}}
	if r := Recovery(before, ok, 1.5); !r.Passed { // 120ms <= 100ms*1.5=150ms
		t.Errorf("recovery should pass: %s", r.Detail)
	}
	bad := metrics.Snapshot{Latency: metrics.LatencyStats{P99: 200 * time.Millisecond}}
	if r := Recovery(before, bad, 1.5); r.Passed { // 200ms > 150ms
		t.Errorf("recovery should fail: %s", r.Detail)
	}
}

func TestDegradation(t *testing.T) {
	first := metrics.Snapshot{Latency: metrics.LatencyStats{P99: 100 * time.Millisecond}}
	if r := Degradation(first, metrics.Snapshot{Latency: metrics.LatencyStats{P99: 120 * time.Millisecond}}, 1.5); !r.Passed {
		t.Errorf("stable soak should pass: %s", r.Detail)
	}
	if r := Degradation(first, metrics.Snapshot{Latency: metrics.LatencyStats{P99: 200 * time.Millisecond}}, 1.5); r.Passed {
		t.Errorf("degraded soak should fail: %s", r.Detail)
	}
}

func TestWithAbortForcesVerdictRed(t *testing.T) {
	passing := model.Verdict{Passed: true, Results: []model.SLOResult{{Name: "p99_under", Passed: true}}}

	if got := WithAbort(passing, false, ""); !got.Passed {
		t.Error("WithAbort(_, false, _) changed a passing verdict; a completed run must be unaffected")
	}

	aborted := WithAbort(passing, true, "kill switch")
	if aborted.Passed {
		t.Error("WithAbort(_, true, _) = passing; an aborted run must not pass (H1)")
	}
	var sawRunResult bool
	for _, r := range aborted.Results {
		if r.Name == "run_completed" && !r.Passed {
			sawRunResult = true
		}
	}
	if !sawRunResult {
		t.Errorf("aborted verdict missing a failed run_completed result: %+v", aborted.Results)
	}
}
