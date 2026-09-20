package blastguard

import (
	"errors"
	"testing"
	"time"

	"github.com/JonasBorgesLM/sapper/internal/core/model"
)

// running returns an admitting guard on a non-production tier, for the tests
// that exercise the admission/halt lifecycle rather than the tier gate.
func running(t *testing.T) *BlastGuard {
	t.Helper()
	g, err := New(model.TierLab, model.Caps{MaxConcurrency: 4, MaxDuration: time.Minute})
	if err != nil {
		t.Fatalf("New(lab) error = %v", err)
	}
	return g
}

func TestAcquireAllowsWhenRunning(t *testing.T) {
	g := running(t)
	release, err := g.Acquire()
	if err != nil {
		t.Fatalf("Acquire() error = %v, want nil for a running guard", err)
	}
	release() // must not panic
}

func TestAcquireDeniesWhenStopped(t *testing.T) {
	g := running(t)
	g.Stop("kill switch")
	_, err := g.Acquire()
	if !errors.Is(err, ErrAborted) {
		t.Fatalf("Acquire() error = %v, want errors.Is(err, ErrAborted)", err)
	}
}

func TestStopIsIdempotentAndKeepsFirstReason(t *testing.T) {
	g := running(t)
	g.Stop("first reason")
	g.Stop("second reason")
	if !g.Stopped() {
		t.Error("Stopped() = false after Stop(), want true")
	}
	if g.Reason() != "first reason" {
		t.Errorf("Reason() = %q, want the first reason recorded", g.Reason())
	}
}

func TestCheckDeniesWhenStopped(t *testing.T) {
	g := running(t)
	if err := g.Check(); err != nil {
		t.Fatalf("Check() error = %v, want nil for a running guard", err)
	}
	g.Stop("halt")
	if err := g.Check(); !errors.Is(err, ErrAborted) {
		t.Fatalf("Check() error = %v, want errors.Is(err, ErrAborted) once stopped", err)
	}
}
