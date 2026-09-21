// Package inject is the fault-injector adapter (ADR-0007): a minimal in-process
// reverse proxy that sits between a target and one of its dependencies and, on
// command, injects an error, added latency, or a dropped connection. It lets a
// scenario fail a dependency without touching the target's code, so Sapper can
// assert a circuit breaker opens and recovers (Case 4, T-05).
//
// It is built with a BlastGuard, whose tier was authorised at construction, so
// a fault cannot be injected against an unauthorised tier; and it stops
// injecting once the guard halts, so a kill switch or auto-abort also quiets the
// faults.
package inject

import (
	"errors"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"

	"github.com/JonasBorgesLM/sapper/internal/core/blastguard"
)

// Kind is the type of fault to inject.
type Kind int

const (
	// None forwards to the upstream unchanged.
	None Kind = iota
	// Error returns Status without contacting the upstream.
	Error
	// Latency waits Delay, then forwards — modelling a slow dependency.
	Latency
	// Drop aborts the connection, modelling a refused/reset dependency.
	Drop
)

// Fault describes the current injection.
type Fault struct {
	Kind   Kind
	Status int
	Delay  time.Duration
}

// Proxy is the fault-injecting reverse proxy. Its zero fault (None) forwards
// cleanly; SetFault switches the injection live during a scenario.
type Proxy struct {
	guard *blastguard.BlastGuard
	rp    *httputil.ReverseProxy

	mu    sync.Mutex
	fault Fault
}

// New builds a proxy forwarding to upstream. guard must not be nil — a fault
// injector without an authorised guard is a programming error (T-05).
func New(guard *blastguard.BlastGuard, upstream string) (*Proxy, error) {
	if guard == nil {
		return nil, errors.New("inject: guard must not be nil — no fault injection without an authorised tier (T-05)")
	}
	u, err := url.Parse(upstream)
	if err != nil || !u.IsAbs() || u.Host == "" {
		return nil, errors.New("inject: upstream must be an absolute URL with a host")
	}
	return &Proxy{guard: guard, rp: httputil.NewSingleHostReverseProxy(u)}, nil
}

// SetFault switches the active fault. It is safe to call during a run.
func (p *Proxy) SetFault(f Fault) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.fault = f
}

func (p *Proxy) current() Fault {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.fault
}

// ServeHTTP applies the active fault, or forwards to the upstream. Once the
// guard has halted, it stops injecting and forwards cleanly, so a kill switch or
// auto-abort quiets the faults as well as the load.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f := p.current()
	if p.guard.Stopped() {
		f.Kind = None
	}
	switch f.Kind {
	case Error:
		w.WriteHeader(f.Status)
	case Latency:
		time.Sleep(f.Delay)
		p.rp.ServeHTTP(w, r)
	case Drop:
		// net/http recovers this, closing the connection without a response —
		// the client sees a reset/EOF, the cleanest std-lib "dropped dependency".
		panic(http.ErrAbortHandler)
	default:
		p.rp.ServeHTTP(w, r)
	}
}
