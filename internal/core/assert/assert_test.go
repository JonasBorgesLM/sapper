package assert

import (
	"testing"
	"time"

	"github.com/JonasBorgesLM/sapper/internal/core/metrics"
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
