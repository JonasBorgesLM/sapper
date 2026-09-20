package scenario

import (
	"strings"
	"testing"
	"time"
)

func TestLoadValid(t *testing.T) {
	s, err := Load("testdata/valid.yaml")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if s.Name != "sustained-baseline" {
		t.Errorf("Name = %q", s.Name)
	}
	if s.Profile.Type != "sustained" || s.Profile.Concurrency != 10 {
		t.Errorf("Profile = %+v", s.Profile)
	}
	if time.Duration(s.Profile.Duration) != 30*time.Second || time.Duration(s.Profile.Warmup) != 5*time.Second {
		t.Errorf("Profile durations = %+v", s.Profile)
	}
	if s.SLOs.P99Under == nil || time.Duration(*s.SLOs.P99Under) != 200*time.Millisecond {
		t.Errorf("SLOs.P99Under = %v", s.SLOs.P99Under)
	}
	if s.SLOs.ErrorRateUnder == nil || *s.SLOs.ErrorRateUnder != 0.01 {
		t.Errorf("SLOs.ErrorRateUnder = %v", s.SLOs.ErrorRateUnder)
	}
}

// #17 / ADR-0001: a scenario with no SLO is refused — Sapper asserts, it does
// not benchmark.
func TestLoadRefusesNoSLOs(t *testing.T) {
	_, err := Load("testdata/no-slos.yaml")
	if err == nil || !strings.Contains(err.Error(), "SLO") {
		t.Fatalf("Load(no-slos) error = %v, want a refusal naming SLOs (ADR-0001)", err)
	}
}

func TestLoadRejectsUnknownProfile(t *testing.T) {
	_, err := Load("testdata/bad-profile.yaml")
	if err == nil || !strings.Contains(err.Error(), "type") {
		t.Fatalf("Load(bad-profile) error = %v, want a refusal naming the profile type", err)
	}
}

func TestLoadRejectsNonPositiveConcurrency(t *testing.T) {
	_, err := Load("testdata/zero-concurrency.yaml")
	if err == nil || !strings.Contains(err.Error(), "concurrency") {
		t.Fatalf("Load(zero-concurrency) error = %v, want a refusal naming concurrency", err)
	}
}
