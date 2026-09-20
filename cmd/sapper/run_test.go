package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/JonasBorgesLM/sapper/internal/adapters/config"
	"github.com/JonasBorgesLM/sapper/internal/core/scenario"
	"github.com/JonasBorgesLM/sapper/internal/core/model"
)

func dur(d time.Duration) *scenario.Duration { sd := scenario.Duration(d); return &sd }

func TestExecuteRunEndToEnd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Config{
		SchemaVersion: 1,
		Target:        config.Target{BaseURL: srv.URL, Tier: model.TierLab},
		BlastRadius:   config.BlastRadius{MaxConcurrency: 5, MaxDuration: config.Duration(time.Minute)},
	}
	sc := &scenario.Scenario{
		Name:    "e2e",
		Profile: scenario.Profile{Type: "sustained", Concurrency: 3, Duration: scenario.Duration(60 * time.Millisecond)},
		SLOs:    scenario.SLOs{P99Under: dur(5 * time.Second)},
	}

	res, err := executeRun(context.Background(), cfg, sc, 2, nil)
	if err != nil {
		t.Fatalf("executeRun() error = %v", err)
	}
	if res.Metrics.Runs != 2 {
		t.Errorf("Metrics.Runs = %d, want 2", res.Metrics.Runs)
	}
	if !res.Verdict.Passed {
		t.Errorf("Verdict.Passed = false, want true (p99 well under 5s); results = %+v", res.Verdict.Results)
	}
	if res.Target.BaseURL != srv.URL || res.Target.Tier != model.TierLab {
		t.Errorf("Target echo = %+v", res.Target)
	}
	if res.Aborted {
		t.Errorf("Aborted = true, want false; reason = %q", res.AbortReason)
	}
}

func TestVerdictExitCode(t *testing.T) {
	if got := verdictExitCode(model.Verdict{Passed: true}); got != 0 {
		t.Errorf("exit for passing verdict = %d, want 0", got)
	}
	if got := verdictExitCode(model.Verdict{Passed: false}); got == 0 {
		t.Errorf("exit for failing verdict = 0, want non-zero (CI gate)")
	}
}
