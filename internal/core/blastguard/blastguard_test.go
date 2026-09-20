package blastguard

import (
	"errors"
	"testing"
)

func TestAcquireAllowsWhenRunning(t *testing.T) {
	g := New()
	release, err := g.Acquire()
	if err != nil {
		t.Fatalf("Acquire() error = %v, want nil for a running guard", err)
	}
	release() // must not panic
}

func TestAcquireDeniesWhenStopped(t *testing.T) {
	g := New()
	g.Stop("kill switch")
	_, err := g.Acquire()
	if !errors.Is(err, ErrAborted) {
		t.Fatalf("Acquire() error = %v, want errors.Is(err, ErrAborted)", err)
	}
}

func TestStopIsIdempotentAndKeepsFirstReason(t *testing.T) {
	g := New()
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
	g := New()
	if err := g.Check(); err != nil {
		t.Fatalf("Check() error = %v, want nil for a running guard", err)
	}
	g.Stop("halt")
	if err := g.Check(); !errors.Is(err, ErrAborted) {
		t.Fatalf("Check() error = %v, want errors.Is(err, ErrAborted) once stopped", err)
	}
}
