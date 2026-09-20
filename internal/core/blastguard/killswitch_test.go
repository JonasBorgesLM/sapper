package blastguard

import (
	"context"
	"testing"

	"github.com/JonasBorgesLM/sapper/internal/core/model"
)

func TestWatchContextStopsGuardOnCancel(t *testing.T) {
	g, err := New(model.TierLab, testCaps)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		WatchContext(ctx, g, "kill switch")
		close(done)
	}()

	if g.Stopped() {
		t.Fatal("guard stopped before any signal")
	}
	cancel() // stands in for SIGINT/SIGTERM cancelling the signal context
	<-done   // WatchContext returns only after it has stopped the guard

	if !g.Stopped() {
		t.Error("guard not stopped after the context was cancelled (SR-04)")
	}
	if g.Reason() != "kill switch" {
		t.Errorf("Reason() = %q, want the kill-switch reason", g.Reason())
	}
}
