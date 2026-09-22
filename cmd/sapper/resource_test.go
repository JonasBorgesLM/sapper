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

// #49: with a metrics endpoint configured, the run samples a target-side value
// once per window and records the series on the result.
func TestExecuteRunSamplesResourceMetric(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			_, _ = w.Write([]byte(`{"goroutines":42}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Config{
		SchemaVersion: 1,
		Target: config.Target{
			BaseURL: srv.URL, Tier: model.TierLab,
			MetricsURL: srv.URL + "/metrics", MetricsField: "goroutines",
		},
		BlastRadius: config.BlastRadius{MaxConcurrency: 5, MaxDuration: config.Duration(time.Minute)},
	}
	sc := &scenario.Scenario{
		Name:    "res",
		Profile: scenario.Profile{Type: "sustained", Concurrency: 2, Duration: scenario.Duration(400 * time.Millisecond)},
		SLOs:    scenario.SLOs{P99Under: dur(5 * time.Second)},
	}
	res, err := executeRun(context.Background(), cfg, sc, 1, nil)
	if err != nil {
		t.Fatalf("executeRun() error = %v", err)
	}
	if len(res.ResourceSamples) == 0 {
		t.Fatal("no resource samples recorded")
	}
	if res.ResourceSamples[0].Value != 42 {
		t.Errorf("sample value = %v, want 42", res.ResourceSamples[0].Value)
	}
}
