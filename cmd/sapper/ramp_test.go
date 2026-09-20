package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JonasBorgesLM/sapper/internal/adapters/config"
	"github.com/JonasBorgesLM/sapper/internal/core/model"
	"github.com/JonasBorgesLM/sapper/internal/core/scenario"
)

// A ramp-up run against a server that starts refusing with 429 past a threshold
// must see the 429 — proof the rate limiter engaged (Case 2, IR-03).
func TestExecuteRunRampUpSeesRateLimit(t *testing.T) {
	var count int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&count, 1) > 20 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Config{
		SchemaVersion: 1,
		Target:        config.Target{BaseURL: srv.URL, Tier: model.TierLab},
		BlastRadius:   config.BlastRadius{MaxConcurrency: 10, MaxDuration: config.Duration(time.Minute)},
	}
	seen := 429
	sc := &scenario.Scenario{
		Name:    "ramp",
		Profile: scenario.Profile{Type: scenario.ProfileRampUp, Concurrency: 10, Duration: scenario.Duration(120 * time.Millisecond)},
		SLOs:    scenario.SLOs{StatusSeen: &seen},
	}

	res, err := executeRun(context.Background(), cfg, sc, 1, nil)
	if err != nil {
		t.Fatalf("executeRun() error = %v", err)
	}
	if res.Metrics.StatusCounts[429] == 0 {
		t.Fatalf("no 429 recorded; status counts = %v", res.Metrics.StatusCounts)
	}
	if !res.Verdict.Passed {
		t.Errorf("verdict failed though 429 was seen: %+v", res.Verdict.Results)
	}
}
