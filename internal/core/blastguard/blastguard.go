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
)

// ErrAborted is returned by Acquire once the guard has been stopped — by the
// kill switch, by the auto-abort ceiling, or by any other halt. A caller that
// receives it must not touch the network.
var ErrAborted = errors.New("blastguard: load aborted")

// BlastGuard is the single point of load admission. It is safe for concurrent
// use: the generator drives it from many goroutines at once.
type BlastGuard struct {
	mu      sync.Mutex
	stopped bool
	reason  string
}

// New builds a running guard. Construction will grow to take the tier and caps
// (SR-01, SR-03); for now a fresh guard admits load until it is stopped.
func New() *BlastGuard {
	return &BlastGuard{}
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
