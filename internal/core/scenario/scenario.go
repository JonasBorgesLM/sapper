// Package scenario defines and loads a scenario file: the load profile and the
// SLOs a run asserts against. It is separate from the safety config (the target
// and BlastGuard's caps) so one safety config can guard many scenarios and a
// scenario can be reused across targets.
//
// A scenario with no SLO is refused (ADR-0001): Sapper asserts against a
// declared expectation, it does not benchmark.
package scenario

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ProfileSustained is the only load profile v1 implements. Others (ramp-up,
// spike, soak) reuse the same engine and are added by their own issues.
const (
	ProfileSustained = "sustained"
	ProfileRampUp    = "ramp-up"
	ProfileSpike     = "spike"
	ProfileSoak      = "soak"
)

// Scenario is the root of a scenario file.
type Scenario struct {
	Name    string  `yaml:"name"`
	Profile Profile `yaml:"profile"`
	Request Request `yaml:"request"`
	SLOs    SLOs    `yaml:"slos"`
}

// Request is the endpoint load is aimed at. Optional; an unset Request means
// GET / against the target's base URL.
type Request struct {
	Method string `yaml:"method"`
	Path   string `yaml:"path"`
}

// Profile is the load shape and its parameters.
type Profile struct {
	Type        string   `yaml:"type"`
	Concurrency int      `yaml:"concurrency"`
	Duration    Duration `yaml:"duration"`
	Warmup      Duration `yaml:"warmup"`

	// Spike-only: the baseline phase run before and after the spike. The spike
	// phase itself uses Concurrency (the peak) for Duration.
	BaselineConcurrency int      `yaml:"baseline_concurrency"`
	BaselineDuration    Duration `yaml:"baseline_duration"`

	// Soak-only: the number of measurement windows the run is split into, to
	// detect slow degradation across them (at least 2).
	Windows int `yaml:"windows"`
}

// SLOs are the constraints a run is asserted against. Each set field is one
// assertion; at least one must be present (ADR-0001). More kinds (status-based)
// are added when ramp-up needs them.
type SLOs struct {
	P99Under       *Duration `yaml:"p99_under"`
	ErrorRateUnder *float64  `yaml:"error_rate_under"`
	StatusSeen     *int      `yaml:"status_seen"`
	// Spike-only: post-spike p99 must be within this factor of the baseline p99.
	RecoveryWithin *float64 `yaml:"recovery_within"`
	// Soak-only: the last window's p99 must be within this factor of the first.
	DegradationUnder *float64 `yaml:"degradation_under"`
}

// Empty reports whether no SLO was declared.
func (s SLOs) Empty() bool {
	return s.P99Under == nil && s.ErrorRateUnder == nil && s.StatusSeen == nil && s.RecoveryWithin == nil && s.DegradationUnder == nil
}

// Duration unmarshals a YAML duration string like "30s".
type Duration time.Duration

// UnmarshalYAML parses a scalar duration string via time.ParseDuration.
func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	*d = Duration(parsed)
	return nil
}

// String renders the duration the "30s" way scenarios use.
func (d Duration) String() string { return time.Duration(d).String() }

// Load reads and validates a scenario file.
func Load(path string) (*Scenario, error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- operator-supplied --scenario path; see httpclient for the same reasoning
	if err != nil {
		return nil, fmt.Errorf("scenario: read %s: %w", path, err)
	}
	var s Scenario
	if err := yaml.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("scenario: parse %s: %w", path, err)
	}
	if err := s.validate(); err != nil {
		return nil, fmt.Errorf("scenario: %s: %w", path, err)
	}
	return &s, nil
}

func (s *Scenario) validate() error {
	var errs []error
	if s.Name == "" {
		errs = append(errs, errors.New("name is required"))
	}
	switch s.Profile.Type {
	case ProfileSustained, ProfileRampUp:
		if s.Profile.Concurrency <= 0 {
			errs = append(errs, fmt.Errorf("profile.concurrency must be a positive integer, got %d", s.Profile.Concurrency))
		}
		if time.Duration(s.Profile.Duration) <= 0 {
			errs = append(errs, errors.New("profile.duration must be a positive duration"))
		}
		if time.Duration(s.Profile.Warmup) < 0 {
			errs = append(errs, errors.New("profile.warmup must not be negative"))
		}
	case ProfileSpike:
		if s.Profile.BaselineConcurrency <= 0 {
			errs = append(errs, fmt.Errorf("profile.baseline_concurrency must be a positive integer, got %d", s.Profile.BaselineConcurrency))
		}
		if time.Duration(s.Profile.BaselineDuration) <= 0 {
			errs = append(errs, errors.New("profile.baseline_duration must be a positive duration"))
		}
		if s.Profile.Concurrency <= 0 {
			errs = append(errs, errors.New("profile.concurrency (the spike peak) must be a positive integer"))
		}
		if time.Duration(s.Profile.Duration) <= 0 {
			errs = append(errs, errors.New("profile.duration (the spike length) must be a positive duration"))
		}
	case ProfileSoak:
		if s.Profile.Concurrency <= 0 {
			errs = append(errs, fmt.Errorf("profile.concurrency must be a positive integer, got %d", s.Profile.Concurrency))
		}
		if time.Duration(s.Profile.Duration) <= 0 {
			errs = append(errs, errors.New("profile.duration must be a positive duration"))
		}
		if s.Profile.Windows < 2 {
			errs = append(errs, fmt.Errorf("profile.windows must be at least 2 to detect degradation, got %d", s.Profile.Windows))
		}
	default:
		errs = append(errs, fmt.Errorf("profile.type %q is not supported (want %q, %q, %q or %q)", s.Profile.Type, ProfileSustained, ProfileRampUp, ProfileSpike, ProfileSoak))
	}
	if s.Request.Path != "" && !strings.HasPrefix(s.Request.Path, "/") {
		errs = append(errs, fmt.Errorf("request.path must start with '/', got %q", s.Request.Path))
	}
	if s.SLOs.Empty() {
		errs = append(errs, errors.New("at least one SLO is required; a run with no SLO is a misconfiguration, not a benchmark (ADR-0001)"))
	}
	return errors.Join(errs...)
}
