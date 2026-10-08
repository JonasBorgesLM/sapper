package inject

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JonasBorgesLM/sapper/internal/core/blastguard"
	"github.com/JonasBorgesLM/sapper/internal/core/model"
)

func upstream(t *testing.T) (*httptest.Server, *int32) {
	t.Helper()
	hits := new(int32)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(hits, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "up")
	}))
	t.Cleanup(srv.Close)
	return srv, hits
}

func newProxy(t *testing.T, upstreamURL string) *Proxy {
	t.Helper()
	g, err := blastguard.New(model.TierLab, model.Caps{MaxConcurrency: 4, MaxDuration: time.Minute})
	if err != nil {
		t.Fatalf("guard: %v", err)
	}
	p, err := New(g, upstreamURL)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return p
}

func TestProxyHealthyForwardsToUpstream(t *testing.T) {
	up, hits := upstream(t)
	front := httptest.NewServer(newProxy(t, up.URL))
	defer front.Close()

	resp, err := http.Get(front.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != 200 || string(body) != "up" || atomic.LoadInt32(hits) != 1 {
		t.Errorf("healthy: status=%d body=%q hits=%d", resp.StatusCode, body, atomic.LoadInt32(hits))
	}
}

func TestProxyErrorFaultShortCircuits(t *testing.T) {
	up, hits := upstream(t)
	p := newProxy(t, up.URL)
	p.SetFault(Fault{Kind: Error, Status: 503})
	front := httptest.NewServer(p)
	defer front.Close()

	resp, err := http.Get(front.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 503 {
		t.Errorf("status = %d, want 503", resp.StatusCode)
	}
	if atomic.LoadInt32(hits) != 0 {
		t.Errorf("upstream hit %d times, want 0 — an error fault must not reach upstream", atomic.LoadInt32(hits))
	}
}

func TestProxyLatencyFaultDelays(t *testing.T) {
	up, _ := upstream(t)
	p := newProxy(t, up.URL)
	p.SetFault(Fault{Kind: Latency, Delay: 60 * time.Millisecond})
	front := httptest.NewServer(p)
	defer front.Close()

	start := time.Now()
	resp, err := http.Get(front.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	_ = resp.Body.Close()
	if elapsed := time.Since(start); elapsed < 50*time.Millisecond {
		t.Errorf("elapsed %s, want >= ~60ms", elapsed)
	}
	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200 (latency still forwards)", resp.StatusCode)
	}
}

func TestProxyDropFaultBreaksConnection(t *testing.T) {
	up, _ := upstream(t)
	p := newProxy(t, up.URL)
	p.SetFault(Fault{Kind: Drop})
	front := httptest.NewServer(p)
	defer front.Close()

	_, err := http.Get(front.URL)
	if err == nil {
		t.Error("Get returned no error, want a connection failure for a dropped request")
	}
}

func TestNewRequiresGuard(t *testing.T) {
	if _, err := New(nil, "http://x"); err == nil {
		t.Error("New(nil, ...) = nil error; a fault injector must be built with an authorised guard (T-05)")
	}
}

// TestProxyStopsInjectingOnceGuardHalted is ADR-0007's own claim made
// concrete: "a fault cannot be injected against an unauthorized tier... the
// same in-process guarantee the load path already has." A kill switch or
// auto-abort calls BlastGuard.Stop, and from that point the proxy must
// forward cleanly rather than keep degrading a real dependency nobody is
// authorising it against anymore.
//
// Negative control: with the `p.guard.Stopped()` check removed from
// ServeHTTP, this test failed — the proxy kept returning the injected fault
// after the guard halted. Verified by hand, restored before committing.
func TestProxyStopsInjectingOnceGuardHalted(t *testing.T) {
	up, hits := upstream(t)
	g, err := blastguard.New(model.TierLab, model.Caps{MaxConcurrency: 4, MaxDuration: time.Minute})
	if err != nil {
		t.Fatalf("guard: %v", err)
	}
	p, err := New(g, up.URL)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	p.SetFault(Fault{Kind: Error, Status: 503})
	front := httptest.NewServer(p)
	defer front.Close()

	g.Stop("test: simulated kill switch")

	resp, err := http.Get(front.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != 200 || string(body) != "up" || atomic.LoadInt32(hits) != 1 {
		t.Errorf("after guard halt: status=%d body=%q hits=%d, want a clean forward to the real upstream", resp.StatusCode, body, atomic.LoadInt32(hits))
	}
}

func TestParseKind(t *testing.T) {
	cases := []struct {
		in      string
		want    Kind
		wantErr bool
	}{
		{"error", Error, false},
		{"latency", Latency, false},
		{"drop", Drop, false},
		{"bogus", None, true},
		{"", None, true},
	}
	for _, c := range cases {
		got, err := ParseKind(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("ParseKind(%q) error = %v, wantErr %v", c.in, err, c.wantErr)
		}
		if got != c.want {
			t.Errorf("ParseKind(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
