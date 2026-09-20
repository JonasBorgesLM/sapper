// Package blastguard implements BlastGuard, the central safety boundary of the
// load generator — the offensive mirror of the scanner's ScopeGuard. Every unit
// of load must pass through it before it reaches the network, and the only path
// to the network any component is given is the guarded client in
// internal/adapters/httpclient. That is what keeps this a resilience harness
// rather than a DoS tool (SR-06).
//
// # What this file covers
//
// The guard's lifecycle: admission (Acquire) and the halt state (Stop). A
// stopped guard denies every subsequent admission, and a denied admission never
// reaches the network. Stop is the primitive the kill switch (SR-04) and the
// catastrophic auto-abort (SR-05) both trigger.
//
// # What it does NOT cover yet
//
// The tier gate (SR-01/SR-02), the concurrency and duration caps (SR-03), and
// the auto-abort ceiling (SR-05) are added to Acquire and construction by their
// own changes. Acquire already returns a release func so concurrency accounting
// can attach without changing this contract.
package blastguard

import (
	"errors"
	"fmt"
	"sync"

	"github.com/JonasBorgesLM/sapper/internal/core/model"
)

// ErrAborted is returned by Acquire once the guard has been stopped — by the
// kill switch, by the auto-abort ceiling, or by any other halt. A caller that
// receives it must not touch the network.
var ErrAborted = errors.New("blastguard: load aborted")

// BlastGuard is the single point of load admission. It is safe for concurrent
// use: the generator drives it from many goroutines at once.
type BlastGuard struct {
	tier model.Tier

	mu      sync.Mutex
	stopped bool
	reason  string
}

// Option configures a guard at construction. It is where the production
// approval (and, later, the caps) are supplied.
type Option func(*config) error

type config struct {
	prodApprovalGiven bool
	confirm           func() (bool, error)
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

// New builds a guard for the given tier, enforcing the tier gate (SR-01/SR-02)
// at construction: it independently refuses an unknown or empty tier (defense
// in depth, not trusting the config loader), and refuses the production tier
// unless WithProductionApproval was supplied AND its interactive confirmation
// succeeds. A guard is only returned when it is authorised to generate load.
func New(tier model.Tier, opts ...Option) (*BlastGuard, error) {
	if !tier.Valid() {
		return nil, fmt.Errorf("blastguard: tier %q is not authorised (want lab|staging|authorized|production) (SR-01)", tier)
	}

	var cfg config
	for _, opt := range opts {
		if err := opt(&cfg); err != nil {
			return nil, err
		}
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

	return &BlastGuard{tier: tier}, nil
}

// Check reports whether the guard is currently open, without admitting a
// request or taking a slot. It is used where a request must be re-validated
// without consuming admission — notably each hop of a redirect chain, which
// net/http follows internally without calling back through the guarded client.
func (g *BlastGuard) Check() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.stopped {
		return fmt.Errorf("%w: %s", ErrAborted, g.reason)
	}
	return nil
}

// Acquire admits one request. It returns an error to deny — and a denied
// request must never reach the network — or a release func the caller invokes
// when the request completes. release is a no-op until concurrency accounting
// attaches to it (SR-03); it is always safe to call exactly once.
func (g *BlastGuard) Acquire() (release func(), err error) {
	if err := g.Check(); err != nil {
		return nil, err
	}
	return func() {}, nil
}

// Stop halts the guard: every subsequent Acquire denies. It is idempotent and
// safe for concurrent use — the first reason wins, so a kill switch that fires
// while an auto-abort is also firing records one coherent cause.
func (g *BlastGuard) Stop(reason string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.stopped {
		g.stopped = true
		g.reason = reason
	}
}

// Stopped reports whether the guard has been halted.
func (g *BlastGuard) Stopped() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.stopped
}

// Reason returns the recorded halt cause, or "" if the guard is still running.
// It is written into the run's result so an aborted run says why (honest data,
// not a hidden failure).
func (g *BlastGuard) Reason() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.reason
}
