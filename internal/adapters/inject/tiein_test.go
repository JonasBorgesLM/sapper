package inject_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/JonasBorgesLM/sapper/internal/adapters/httpclient"
	"github.com/JonasBorgesLM/sapper/internal/adapters/inject"
	"github.com/JonasBorgesLM/sapper/internal/core/blastguard"
	"github.com/JonasBorgesLM/sapper/internal/core/generator"
	"github.com/JonasBorgesLM/sapper/internal/core/metrics"
	"github.com/JonasBorgesLM/sapper/internal/core/model"
)

// breaker is a minimal circuit breaker standing in for bastion (IR-02: depend on
// the property — opens after N consecutive failures, half-opens after a cooldown
// — not the library). It fast-fails while open instead of calling the dependency.
type breaker struct {
	mu        sync.Mutex
	failures  int
	openUntil time.Time
	threshold int
	cooldown  time.Duration
}

var errOpen = errors.New("breaker open")

func (b *breaker) call(fn func() error) error {
	b.mu.Lock()
	if time.Now().Before(b.openUntil) {
		b.mu.Unlock()
		return errOpen
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

// Case 4: inject a dependency fault, assert the breaker opens (fast-fails with
// 503 rather than cascading) and recovers to 200 once the fault clears.
func TestBastionStyleBreakerOpensAndRecovers(t *testing.T) {
	dependency := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer dependency.Close()

	guard, err := blastguard.New(model.TierLab, model.Caps{MaxConcurrency: 4, MaxDuration: time.Minute})
	if err != nil {
		t.Fatalf("guard: %v", err)
	}
	proxy, err := inject.New(guard, dependency.URL)
	if err != nil {
		t.Fatalf("inject.New: %v", err)
	}
	depFront := httptest.NewServer(proxy)
	defer depFront.Close()

	// The bastion-wrapped service: each request calls its dependency through the
	// breaker. Open -> 503 (graceful fast-fail); dependency error -> 502; ok -> 200.
	br := &breaker{threshold: 3, cooldown: 60 * time.Millisecond}
	service := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		err := br.call(func() error {
			resp, err := http.Get(depFront.URL)
			if err != nil {
				return err
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode >= 500 {
				return errors.New("dependency 5xx")
			}
			return nil
		})
		switch {
		case errors.Is(err, errOpen):
			w.WriteHeader(http.StatusServiceUnavailable) // 503: breaker open
		case err != nil:
			w.WriteHeader(http.StatusBadGateway) // 502: dependency failing
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer service.Close()

	client := httpclient.New(guard, nil)
	newReq := func(ctx context.Context) (*http.Request, error) {
		return http.NewRequestWithContext(ctx, http.MethodGet, service.URL, nil)
	}
	phase := func() metrics.Snapshot {
		coll := metrics.New()
		_ = generator.RunSustained(context.Background(), client, coll, newReq, generator.Sustained{Concurrency: 2, Duration: 80 * time.Millisecond})
		return coll.Snapshot()
	}

	// Fault phase: dependency returns 503 -> breaker should open (fast-fail 503).
	proxy.SetFault(inject.Fault{Kind: inject.Error, Status: 503})
	faulted := phase()
	if faulted.StatusCounts[503] == 0 {
		t.Fatalf("breaker did not open under the injected fault; statuses = %v", faulted.StatusCounts)
	}

	// Recovery phase: clear the fault, wait out the cooldown -> breaker recovers.
	proxy.SetFault(inject.Fault{Kind: inject.None})
	time.Sleep(70 * time.Millisecond)
	recovered := phase()
	if recovered.StatusCounts[200] == 0 {
		t.Fatalf("service did not recover after the fault cleared; statuses = %v", recovered.StatusCounts)
	}
}
