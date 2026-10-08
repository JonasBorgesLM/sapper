package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/JonasBorgesLM/sapper/internal/adapters/config"
	"github.com/JonasBorgesLM/sapper/internal/core/model"
	"github.com/JonasBorgesLM/sapper/internal/core/scenario"
)

// freePort picks a free loopback address for the injector to listen on —
// the fault-injection profile needs a known, fixed address (ADR-0007), so
// unlike the other profiles' tests this one cannot hand the target an
// httptest.Server URL and call it done; something has to claim a real port
// for the scenario to name.
func freePort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("freePort: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr
}

// TestExecuteFaultInjectionBreakerOpensAndRecovers is Case 4 end to end
// through the real CLI path: a target with its own minimal breaker, talking
// to a dependency routed through the fault injector, sees the breaker open
// (status_seen) under the injected fault and recover (recovery_within)
// once it clears — the same property internal/adapters/inject's own
// tiein_test.go proves for the generator directly, proven again here
// through scenario.Load + executeRun, the path an operator actually uses.
func TestExecuteFaultInjectionBreakerOpensAndRecovers(t *testing.T) {
	dependency := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer dependency.Close()

	listen := freePort(t)

	// The target under test: a minimal breaker in front of a call to its
	// dependency, routed through whatever sits at `listen` — exactly the
	// shape ADR-0007 describes an operator arranging for their own target.
	br := &testBreaker{threshold: 2, cooldown: 60 * time.Millisecond}
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		err := br.call(func() error {
			resp, err := http.Get("http://" + listen)
			if err != nil {
				return err
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode >= 500 {
				return fmt.Errorf("dependency %d", resp.StatusCode)
			}
			return nil
		})
		switch {
		case errors.Is(err, errBreakerOpen):
			w.WriteHeader(http.StatusServiceUnavailable)
		case err != nil:
			w.WriteHeader(http.StatusBadGateway)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer target.Close()

	cfg := &config.Config{
		SchemaVersion: 1,
		Target:        config.Target{BaseURL: target.URL, Tier: model.TierLab},
		BlastRadius:   config.BlastRadius{MaxConcurrency: 20, MaxDuration: config.Duration(time.Minute)},
	}
	statusSeen := 503
	recovery := 5.0
	sc := &scenario.Scenario{
		Name: "fault-injection-breaker",
		Profile: scenario.Profile{
			Type:                scenario.ProfileFaultInjection,
			BaselineConcurrency: 2,
			BaselineDuration:    scenario.Duration(80 * time.Millisecond),
			Concurrency:         4,
			Duration:            scenario.Duration(100 * time.Millisecond),
			Upstream:            dependency.URL,
			Listen:              listen,
			FaultKind:           "error",
			FaultStatus:         503,
		},
		SLOs: scenario.SLOs{StatusSeen: &statusSeen, RecoveryWithin: &recovery},
	}

	res, err := executeRun(context.Background(), cfg, sc, 1, nil)
	if err != nil {
		t.Fatalf("executeRun() error = %v", err)
	}
	if res.Profile != scenario.ProfileFaultInjection {
		t.Errorf("Profile = %q, want %q", res.Profile, scenario.ProfileFaultInjection)
	}
	var sawStatusSeen, sawRecovery bool
	for _, r := range res.Verdict.Results {
		switch r.Name {
		case "status_seen":
			sawStatusSeen = true
		case "recovery_within":
			sawRecovery = true
		}
	}
	if !sawStatusSeen {
		t.Error("verdict has no status_seen result")
	}
	if !sawRecovery {
		t.Error("verdict has no recovery_within result")
	}
	if !res.Verdict.Passed {
		t.Errorf("verdict failed: %+v", res.Verdict.Results)
	}
}

// testBreaker mirrors internal/adapters/inject/tiein_test.go's own breaker —
// duplicated rather than exported from a test file, which Go cannot import
// across packages anyway; kept deliberately tiny, the same reasoning that
// file gives for standing in for bastion rather than depending on it. The
// mutex matters here in a way it did not in that file's own single-flight
// style: this test's target.URL is hit by Concurrency goroutines at once
// (sapper's own generator, not a single sequential caller), so every field
// read or write below is genuinely concurrent.
type testBreaker struct {
	mu        sync.Mutex
	failures  int
	openUntil time.Time
	threshold int
	cooldown  time.Duration
}

var errBreakerOpen = errors.New("breaker open")

func (b *testBreaker) call(fn func() error) error {
	b.mu.Lock()
	if time.Now().Before(b.openUntil) {
		b.mu.Unlock()
		return errBreakerOpen
	}
	b.mu.Unlock()

	err := fn()

	b.mu.Lock()
	defer b.mu.Unlock()
	if err != nil {
		b.failures++
		if b.failures >= b.threshold {
			b.openUntil = time.Now().Add(b.cooldown)
		}
		return err
	}
	b.failures = 0
	return nil
}
