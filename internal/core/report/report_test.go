package report

import (
	"strings"
	"testing"
	"time"

	"github.com/JonasBorgesLM/sapper/internal/core/metrics"
	"github.com/JonasBorgesLM/sapper/internal/core/model"
)

func TestRenderReportContainsKeyFacts(t *testing.T) {
	result := model.Result{
		Target:   model.TargetEcho{BaseURL: "http://localhost:8080", Tier: model.TierLab, Caps: model.Caps{MaxConcurrency: 10, MaxDuration: time.Minute}},
		Scenario: "sustained-baseline",
		Profile:  "sustained",
		Metrics: metrics.Aggregation{
			Runs:      3,
			P99:       metrics.DurationStat{Mean: 150 * time.Millisecond, StdDev: 10 * time.Millisecond, Min: 140 * time.Millisecond, Max: 165 * time.Millisecond},
			ErrorRate: metrics.RateStat{Mean: 0.005},
		},
		Verdict: model.Verdict{Passed: true, Results: []model.SLOResult{{Name: "p99_under", Passed: true, Detail: "p99 mean 150ms < 200ms"}}},
	}

	html, err := Render(result)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	for _, want := range []string{"sustained-baseline", "sustained", "150ms", "lab", "PASS", "p99_under", "<html"} {
		if !strings.Contains(html, want) {
			t.Errorf("report missing %q", want)
		}
	}
	// "no network": the report must not reference external resources.
	for _, bad := range []string{"src=", "https://", "cdn"} {
		if strings.Contains(html, bad) {
			t.Errorf("report references an external resource (%q); it must be self-contained", bad)
		}
	}
}

func TestRenderReportShowsFailAndAbort(t *testing.T) {
	result := model.Result{
		Scenario: "s", Profile: "sustained",
		Aborted: true, AbortReason: "auto-abort: error rate 90% over the 50% ceiling",
		Metrics: metrics.Aggregation{Runs: 1},
		Verdict: model.Verdict{Passed: false, Results: []model.SLOResult{{Name: "error_rate_under", Passed: false, Detail: "over"}}},
	}
	html, err := Render(result)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if !strings.Contains(html, "FAIL") || !strings.Contains(html, "auto-abort") {
		t.Errorf("report should show FAIL and the abort reason")
	}
}
