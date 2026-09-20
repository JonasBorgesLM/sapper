package blastguard

import (
	"errors"
	"testing"
	"time"

	"github.com/JonasBorgesLM/sapper/internal/core/model"
)

var testCaps = model.Caps{MaxConcurrency: 4, MaxDuration: time.Minute}

func TestNewRejectsNonPositiveCaps(t *testing.T) {
	cases := []model.Caps{
		{MaxConcurrency: 0, MaxDuration: time.Minute},
		{MaxConcurrency: 4, MaxDuration: 0},
	}
	for _, caps := range cases {
		if _, err := New(model.TierLab, caps); err == nil {
			t.Errorf("New(lab, %+v) = nil, want an error (SR-03)", caps)
		}
	}
}

func TestAcquireEnforcesConcurrencyCeiling(t *testing.T) {
	g, err := New(model.TierLab, model.Caps{MaxConcurrency: 2, MaxDuration: time.Minute})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	r1, err := g.Acquire()
	if err != nil {
		t.Fatalf("Acquire() #1 error = %v", err)
	}
	if _, err := g.Acquire(); err != nil {
		t.Fatalf("Acquire() #2 error = %v", err)
	}
	// Two slots are taken; the ceiling is 2, so the third must be refused.
	if _, err := g.Acquire(); err == nil {
		t.Fatal("Acquire() #3 = nil, want refusal at the concurrency ceiling (SR-03)")
	}
	// Freeing one slot lets the next through.
	r1()
	if _, err := g.Acquire(); err != nil {
		t.Errorf("Acquire() after release error = %v, want a freed slot", err)
	}
}

func TestAcquireDeniesAfterMaxDuration(t *testing.T) {
	now := time.Unix(0, 0)
	clock := func() time.Time { return now }
	g, err := New(model.TierLab, model.Caps{MaxConcurrency: 4, MaxDuration: 10 * time.Second}, WithClock(clock))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, err := g.Acquire(); err != nil {
		t.Fatalf("Acquire() before the deadline error = %v, want nil", err)
	}
	now = time.Unix(0, 0).Add(11 * time.Second) // past max_duration
	if _, err := g.Acquire(); !errors.Is(err, ErrAborted) {
		t.Fatalf("Acquire() past the deadline = %v, want errors.Is(err, ErrAborted) (SR-03)", err)
	}
	if !g.Stopped() {
		t.Error("guard not Stopped() after exceeding max_duration")
	}
}
