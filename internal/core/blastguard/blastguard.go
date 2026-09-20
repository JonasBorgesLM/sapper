// Package blastguard implements BlastGuard, the central safety boundary of the
// load generator — the offensive mirror of the scanner's ScopeGuard. Every unit
// of load must pass through it before it reaches the network, and the only path
// to the network any component is given is the guarded client in
// internal/adapters/httpclient. That is what keeps this a resilience harness
// rather than a DoS tool (SR-06).
//
// The guard enforces, at admission: the environment tier (SR-01/SR-02, checked
// at construction), the mandatory concurrency and duration caps (SR-03), and
// the halt state that the kill switch (SR-04) and auto-abort (SR-05) trigger.
//
// The concurrency cap is a backstop: the generator paces itself to the ceiling,
// and the guard guarantees no more than the ceiling are ever in flight even if
// the generator misbehaves — so exceeding it is refused, not queued.
package blastguard

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/JonasBorgesLM/sapper/internal/core/model"
)

// ErrAborted is returned by Acquire/Check once the guard has halted — the kill
// switch, the auto-abort ceiling, or the max-duration cap. A caller that
// receives it must not touch the network.
var ErrAborted = errors.New("blastguard: load aborted")

// ErrConcurrencyExceeded is returned when admitting a request would exceed the
// concurrency cap. Unlike ErrAborted it is transient — a slot may free up — and
// signals the generator oversubscribed, since it is meant to pace itself below
// the ceiling (SR-03).
var ErrConcurrencyExceeded = errors.New("blastguard: concurrency cap exceeded")

// BlastGuard is the single point of load admission. It is safe for concurrent
// use: the generator drives it from many goroutines at once.
type BlastGuard struct {
	tier     model.Tier
	caps     model.Caps
	now      func() time.Time
	deadline time.Time

	mu       sync.Mutex
	stopped  bool
	reason   string
	inFlight int
}

// Option configures a guard at construction.
type Option func(*config) error

type config struct {
	prodApprovalGiven bool
	confirm           func() (bool, error)
	now               func() time.Time
}

// WithProductionApproval opts a run in to the production tier. It stands for the
// operator's explicit, noisy flag; confirm is the interactive confirmation
// (SR-02). A nil confirm models a non-interactive context (CI), where
// production is refused because no human confirmation can be obtained.
func WithProductionApproval(confirm func() (bool, error)) Option {
	return func(c *config) error {
		c.prodApprovalGiven = true
		c.confirm = confirm
		return nil
	}
}

// WithClock injects the time source, so the duration cap is testable without
// waiting. It defaults to time.Now.
func WithClock(now func() time.Time) Option {
	return func(c *config) error {
		c.now = now
		return nil
	}
}

// New builds a guard for the given tier and caps, enforcing the tier gate
// (SR-01/SR-02) and validating the caps (SR-03) at construction — independently
// of the config loader (defense in depth). A guard is only returned when it is
// authorised to generate load. The duration cap starts running now.
func New(tier model.Tier, caps model.Caps, opts ...Option) (*BlastGuard, error) {
	var cfg config
	for _, opt := range opts {
		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}
	if cfg.now == nil {
		cfg.now = time.Now
	}

	if !tier.Valid() {
		return nil, fmt.Errorf("blastguard: tier %q is not authorised (want lab|staging|authorized|production) (SR-01)", tier)
	}
	if err := caps.Validate(); err != nil {
		return nil, fmt.Errorf("blastguard: %w (SR-03)", err)
	}

	if tier.IsProduction() {
		if !cfg.prodApprovalGiven {
			return nil, errors.New("blastguard: the production tier requires explicit approval (the production flag) (SR-02)")
		}
		if cfg.confirm == nil {
			return nil, errors.New("blastguard: the production tier requires interactive confirmation; refusing in a non-interactive context (SR-02)")
		}
		ok, err := cfg.confirm()
		if err != nil {
			return nil, fmt.Errorf("blastguard: production confirmation failed: %w", err)
		}
		if !ok {
			return nil, errors.New("blastguard: production run not confirmed; aborting (SR-02)")
		}
	}

	return &BlastGuard{
		tier:     tier,
		caps:     caps,
		now:      cfg.now,
		deadline: cfg.now().Add(caps.MaxDuration),
	}, nil
}

// admitLocked reports whether load may be emitted right now. It halts the guard
// when the duration cap is reached, so the cause is recorded once. Caller holds
// g.mu.
func (g *BlastGuard) admitLocked() error {
	if g.stopped {
		return fmt.Errorf("%w: %s", ErrAborted, g.reason)
	}
	if !g.now().Before(g.deadline) {
		g.stopped = true
		g.reason = fmt.Sprintf("max_duration %s exceeded", g.caps.MaxDuration)
		return fmt.Errorf("%w: %s", ErrAborted, g.reason)
	}
	return nil
}

// Check reports whether the guard is currently open, without admitting a
// request or taking a concurrency slot — used where a request must be
// re-validated without consuming admission, such as each redirect hop.
func (g *BlastGuard) Check() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.admitLocked()
}

// Acquire admits one request. It denies — and a denied request must never reach
// the network — when the guard has halted (ErrAborted) or the concurrency cap
// is full (ErrConcurrencyExceeded). On success it returns a release func the
// caller invokes when the request completes; release is idempotent.
func (g *BlastGuard) Acquire() (release func(), err error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if err := g.admitLocked(); err != nil {
		return nil, err
	}
	if g.inFlight >= g.caps.MaxConcurrency {
		return nil, fmt.Errorf("%w: %d of %d in flight", ErrConcurrencyExceeded, g.inFlight, g.caps.MaxConcurrency)
	}
	g.inFlight++
	return g.release(), nil
}

func (g *BlastGuard) release() func() {
	var once sync.Once
	return func() {
		once.Do(func() {
			g.mu.Lock()
			g.inFlight--
			g.mu.Unlock()
		})
	}
}

// Stop halts the guard: every subsequent Acquire/Check denies. Idempotent and
// safe for concurrent use — the first reason wins, so a kill switch firing while
// an auto-abort also fires records one coherent cause.
func (g *BlastGuard) Stop(reason string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.stopped {
		g.stopped = true
		g.reason = reason
	}
}

// Stopped reports whether the guard has halted.
func (g *BlastGuard) Stopped() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.stopped
}

// Reason returns the recorded halt cause, or "" if the guard is still running.
// It is written into the run's result so an aborted run says why.
func (g *BlastGuard) Reason() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.reason
}
