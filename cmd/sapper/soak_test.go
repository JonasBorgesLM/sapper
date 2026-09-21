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

// A soak against a healthy, stable server: p99 does not degrade across the
// windows, so the degradation SLO passes (Case 5).
func TestExecuteSoakStable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Config{
		SchemaVersion: 1,
		Target:        config.Target{BaseURL: srv.URL, Tier: model.TierLab},
		BlastRadius:   config.BlastRadius{MaxConcurrency: 10, MaxDuration: config.Duration(time.Minute)},
	}
	factor := 4.0
	sc := &scenario.Scenario{
		Name:    "soak",
		Profile: scenario.Profile{Type: scenario.ProfileSoak, Concurrency: 3, Duration: scenario.Duration(150 * time.Millisecond), Windows: 3},
		SLOs:    scenario.SLOs{DegradationUnder: &factor},
	}

	res, err := executeRun(context.Background(), cfg, sc, 1, nil)
	if err != nil {
		t.Fatalf("executeRun() error = %v", err)
	}
	if res.Profile != "soak" {
		t.Errorf("Profile = %q, want soak", res.Profile)
	}
	var sawDegradation bool
	for _, r := range res.Verdict.Results {
		if r.Name == "degradation_under" {
			sawDegradation = true
		}
	}
	if !sawDegradation {
		t.Error("verdict has no degradation_under result")
	}
	if !res.Verdict.Passed {
		t.Errorf("stable soak verdict failed: %+v", res.Verdict.Results)
	}
}
