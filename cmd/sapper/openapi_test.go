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

func specConfig(t *testing.T, baseURL string) *config.Config {
	t.Helper()
	return &config.Config{
		SchemaVersion: 1,
		Target:        config.Target{BaseURL: baseURL, Tier: model.TierLab, OpenAPISpec: "testdata/spec.yaml"},
		BlastRadius:   config.BlastRadius{MaxConcurrency: 5, MaxDuration: config.Duration(time.Minute)},
	}
}

func TestExecuteRunTargetsImportedEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ping" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	sc := &scenario.Scenario{
		Name:    "ping",
		Profile: scenario.Profile{Type: "sustained", Concurrency: 2, Duration: scenario.Duration(50 * time.Millisecond)},
		Request: scenario.Request{Method: "GET", Path: "/ping"},
		SLOs:    scenario.SLOs{P99Under: dur(5 * time.Second)},
	}
	res, err := executeRun(context.Background(), specConfig(t, srv.URL), sc, 1, nil)
	if err != nil {
		t.Fatalf("executeRun() error = %v", err)
	}
	if res.Metrics.StatusCounts[200] == 0 || res.Metrics.StatusCounts[404] != 0 {
		t.Errorf("expected all 200s from /ping, got %v", res.Metrics.StatusCounts)
	}
}

func TestExecuteRunRejectsEndpointNotInSpec(t *testing.T) {
	sc := &scenario.Scenario{
		Name:    "missing",
		Profile: scenario.Profile{Type: "sustained", Concurrency: 2, Duration: scenario.Duration(50 * time.Millisecond)},
		Request: scenario.Request{Method: "GET", Path: "/not-in-spec"},
		SLOs:    scenario.SLOs{P99Under: dur(time.Second)},
	}
	_, err := executeRun(context.Background(), specConfig(t, "http://127.0.0.1:9"), sc, 1, nil)
	if err == nil {
		t.Fatal("executeRun ran a request not declared in the spec (IR-01 validation)")
	}
}
