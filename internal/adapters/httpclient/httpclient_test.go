package httpclient

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JonasBorgesLM/sapper/internal/core/blastguard"
	"github.com/JonasBorgesLM/sapper/internal/core/model"
	"github.com/JonasBorgesLM/sapper/internal/ports"
)

// compile-time proof the guarded client is the ports.Requester implementation.
var _ ports.Requester = (*Client)(nil)

func openGuard(t *testing.T) *blastguard.BlastGuard {
	t.Helper()
	g, err := blastguard.New(model.TierLab, model.Caps{MaxConcurrency: 4, MaxDuration: time.Minute})
	if err != nil {
		t.Fatalf("blastguard.New(lab) error = %v", err)
	}
	return g
}

func countingServer(t *testing.T) (*httptest.Server, *int32) {
	t.Helper()
	hits := new(int32)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(hits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv, hits
}

func TestDoReachesServerWhenGuardIsOpen(t *testing.T) {
	srv, hits := countingServer(t)
	client := New(openGuard(t), nil)

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do() error = %v, want nil for an open guard", err)
	}
	_ = resp.Body.Close()
	if got := atomic.LoadInt32(hits); got != 1 {
		t.Errorf("server received %d requests, want 1", got)
	}
}

// SR-06: when the guard denies, the request must never reach the network.
func TestDoBlockedRequestNeverReachesServer(t *testing.T) {
	srv, hits := countingServer(t)
	guard := openGuard(t)
	guard.Stop("kill switch")
	client := New(guard, nil)

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	resp, err := client.Do(req)
	if resp != nil {
		_ = resp.Body.Close()
		t.Errorf("Do() response = %v, want nil when blocked", resp)
	}
	if !errors.Is(err, blastguard.ErrAborted) {
		t.Fatalf("Do() error = %v, want errors.Is(err, blastguard.ErrAborted)", err)
	}
	if got := atomic.LoadInt32(hits); got != 0 {
		t.Errorf("server received %d requests, want 0 — a blocked request must never touch the network", got)
	}
}

func TestNewPanicsOnNilGuard(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("New(nil, ...) did not panic; there must be no way to build an unguarded client")
		}
	}()
	New(nil, nil)
}
