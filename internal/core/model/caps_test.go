package model

import (
	"strings"
	"testing"
	"time"
)

func TestCapsValidateAcceptsPositive(t *testing.T) {
	c := Caps{MaxConcurrency: 50, MaxDuration: 60 * time.Second}
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

func TestCapsValidateRejectsNonPositiveConcurrency(t *testing.T) {
	for _, n := range []int{0, -1} {
		c := Caps{MaxConcurrency: n, MaxDuration: time.Second}
		err := c.Validate()
		if err == nil {
			t.Fatalf("MaxConcurrency=%d: Validate() = nil, want error", n)
		}
		if !strings.Contains(err.Error(), "max_concurrency") {
			t.Errorf("MaxConcurrency=%d: error %q does not name the field", n, err)
		}
	}
}

func TestCapsValidateRejectsNonPositiveDuration(t *testing.T) {
	for _, d := range []time.Duration{0, -time.Second} {
		c := Caps{MaxConcurrency: 1, MaxDuration: d}
		err := c.Validate()
		if err == nil {
			t.Fatalf("MaxDuration=%s: Validate() = nil, want error", d)
		}
		if !strings.Contains(err.Error(), "max_duration") {
			t.Errorf("MaxDuration=%s: error %q does not name the field", d, err)
		}
	}
}
