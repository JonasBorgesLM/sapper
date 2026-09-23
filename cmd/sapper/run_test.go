package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/JonasBorgesLM/sapper/internal/adapters/config"
	"github.com/JonasBorgesLM/sapper/internal/core/model"
	"github.com/JonasBorgesLM/sapper/internal/core/scenario"
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

// A target that needs auth has no login flow to drive here yet (sapper has
// none), so a scenario configures the header directly, e.g. Authorization.
// Every request the generator sends must carry it.
func TestExecuteRunSendsConfiguredHeaders(t *testing.T) {
	var mu sync.Mutex
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		got = r.Header.Get("Authorization")
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Config{
		SchemaVersion: 1,
		Target:        config.Target{BaseURL: srv.URL, Tier: model.TierLab},
		BlastRadius:   config.BlastRadius{MaxConcurrency: 5, MaxDuration: config.Duration(time.Minute)},
	}
	sc := &scenario.Scenario{
		Name:    "headers",
		Profile: scenario.Profile{Type: "sustained", Concurrency: 2, Duration: scenario.Duration(30 * time.Millisecond)},
		Request: scenario.Request{Headers: map[string]string{"Authorization": "Bearer test-token"}},
		SLOs:    scenario.SLOs{P99Under: dur(5 * time.Second)},
	}

	res, err := executeRun(context.Background(), cfg, sc, 1, nil)
	if err != nil {
		t.Fatalf("executeRun() error = %v", err)
	}
	if res.Aborted {
		t.Fatalf("Aborted = true, reason = %q", res.AbortReason)
	}
	mu.Lock()
	defer mu.Unlock()
	if got != "Bearer test-token" {
		t.Errorf("target received Authorization = %q, want %q", got, "Bearer test-token")
	}
}

// H1: a run halted by a cap/kill/auto-abort must not report a green verdict.
func TestExecuteRunAbortedRunIsNotGreen(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Config{
		SchemaVersion: 1,
		Target:        config.Target{BaseURL: srv.URL, Tier: model.TierLab},
		// A 1ms duration cap trips almost immediately, aborting the run.
		BlastRadius: config.BlastRadius{MaxConcurrency: 5, MaxDuration: config.Duration(time.Millisecond)},
	}
	sc := &scenario.Scenario{
		Name:    "aborts",
		Profile: scenario.Profile{Type: "sustained", Concurrency: 2, Duration: scenario.Duration(200 * time.Millisecond)},
		SLOs:    scenario.SLOs{P99Under: dur(5 * time.Second)},
	}
	res, err := executeRun(context.Background(), cfg, sc, 1, nil)
	if err != nil {
		t.Fatalf("executeRun() error = %v", err)
	}
	if !res.Aborted {
		t.Fatalf("expected the run to abort on the duration cap; reason=%q", res.AbortReason)
	}
	if res.Verdict.Passed {
		t.Errorf("aborted run reported a green verdict (H1); results = %+v", res.Verdict.Results)
	}
}

// M3: a profile concurrency above the blast-radius cap is a misconfiguration
// (it would make surplus workers busy-loop on ErrConcurrencyExceeded).
func TestExecuteRunRejectsConcurrencyOverCap(t *testing.T) {
	cfg := &config.Config{
		SchemaVersion: 1,
		Target:        config.Target{BaseURL: "http://127.0.0.1:9", Tier: model.TierLab},
		BlastRadius:   config.BlastRadius{MaxConcurrency: 5, MaxDuration: config.Duration(time.Minute)},
	}
	sc := &scenario.Scenario{
		Name:    "over",
		Profile: scenario.Profile{Type: "sustained", Concurrency: 50, Duration: scenario.Duration(50 * time.Millisecond)},
		SLOs:    scenario.SLOs{P99Under: dur(time.Second)},
	}
	if _, err := executeRun(context.Background(), cfg, sc, 1, nil); err == nil {
		t.Fatal("executeRun ran with concurrency 50 over a cap of 5 (M3)")
	}
}
