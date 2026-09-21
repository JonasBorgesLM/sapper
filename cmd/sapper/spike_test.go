package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/JonasBorgesLM/sapper/internal/adapters/config"
	"github.com/JonasBorgesLM/sapper/internal/core/model"
	"github.com/JonasBorgesLM/sapper/internal/core/scenario"
)

// A spike against a healthy server: latency returns to the baseline after the
// peak, so the recovery SLO passes (Case 3).
func TestExecuteSpikeRecovers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Config{
		SchemaVersion: 1,
		Target:        config.Target{BaseURL: srv.URL, Tier: model.TierLab},
		BlastRadius:   config.BlastRadius{MaxConcurrency: 20, MaxDuration: config.Duration(time.Minute)},
	}
	factor := 5.0
	sc := &scenario.Scenario{
		Name: "spike",
		Profile: scenario.Profile{
			Type:                scenario.ProfileSpike,
			BaselineConcurrency: 2,
			BaselineDuration:    scenario.Duration(60 * time.Millisecond),
			Concurrency:         12,
			Duration:            scenario.Duration(60 * time.Millisecond),
		},
		SLOs: scenario.SLOs{RecoveryWithin: &factor},
	}

	res, err := executeRun(context.Background(), cfg, sc, 1, nil)
	if err != nil {
		t.Fatalf("executeRun() error = %v", err)
	}
	if res.Profile != "spike" {
		t.Errorf("Profile = %q, want spike", res.Profile)
	}
	var sawRecovery bool
	for _, r := range res.Verdict.Results {
		if r.Name == "recovery_within" {
			sawRecovery = true
		}
	}
	if !sawRecovery {
		t.Error("verdict has no recovery_within result")
	}
	if !res.Verdict.Passed {
		t.Errorf("recovery verdict failed against a healthy server: %+v", res.Verdict.Results)
	}
}
