package generator

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/JonasBorgesLM/sapper/internal/adapters/httpclient"
	"github.com/JonasBorgesLM/sapper/internal/core/blastguard"
	"github.com/JonasBorgesLM/sapper/internal/core/metrics"
	"github.com/JonasBorgesLM/sapper/internal/core/model"
)

func harness(t *testing.T, h http.HandlerFunc) (*httpclient.Client, *blastguard.BlastGuard, *metrics.Collector, RequestFunc) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	g, err := blastguard.New(model.TierLab, model.Caps{MaxConcurrency: 8, MaxDuration: time.Minute})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	client := httpclient.New(g, nil)
	coll := metrics.New()
	newReq := func(ctx context.Context) (*http.Request, error) {
		return http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	}
	return client, g, coll, newReq
}

func TestRunSustainedProducesRecords(t *testing.T) {
	client, _, coll, newReq := harness(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	err := RunSustained(context.Background(), client, coll, newReq, Sustained{Concurrency: 3, Duration: 60 * time.Millisecond})
	if err != nil {
		t.Fatalf("RunSustained() error = %v", err)
	}
	s := coll.Snapshot()
	if s.Total == 0 {
		t.Fatal("no requests recorded")
	}
	if s.Errors != 0 {
		t.Errorf("Errors = %d, want 0 against a healthy server", s.Errors)
	}
	if s.StatusCounts[200] != s.Total {
		t.Errorf("StatusCounts[200] = %d, want all %d", s.StatusCounts[200], s.Total)
	}
}

func TestRunSustainedStopsWhenGuardHalted(t *testing.T) {
	client, g, coll, newReq := harness(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	g.Stop("kill switch") // halted before the run starts

	done := make(chan struct{})
	go func() {
		_ = RunSustained(context.Background(), client, coll, newReq, Sustained{Concurrency: 3, Duration: time.Minute})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RunSustained did not return promptly after the guard halted")
	}
	if got := coll.Snapshot().Total; got != 0 {
		t.Errorf("recorded %d requests, want 0 — a halted guard admits nothing", got)
	}
}
