// Package config loads and validates the safety config that drives every
// subcommand: the target, its environment tier, the mandatory blast-radius
// caps, and optional auth. It is an adapter (it reads files and the process
// environment); the core is handed an already-validated Config.
//
// The tier and both caps are required with no default — a missing one aborts
// the load (SR-01, SR-03). Secrets are written as ${VAR} and expanded from the
// environment (SR-07); an unset variable is an error, not a silent literal.
package config

import (
	"errors"
	"fmt"
	"maps"
	"net/url"
	"os"
	"slices"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/JonasBorgesLM/sapper/internal/core/model"
	"github.com/JonasBorgesLM/sapper/internal/envexpand"
)

// SupportedSchemaVersion is the only schema_version this package understands.
// Load rejects anything else so a future format change fails loudly rather than
// being silently misread.
const SupportedSchemaVersion = 1

// Config is the root of the safety config file.
type Config struct {
	SchemaVersion int         `yaml:"schema_version"`
	Target        Target      `yaml:"target"`
	BlastRadius   BlastRadius `yaml:"blast_radius"`
	Auth          Auth        `yaml:"auth"`
}

// Target is the API under test and its environment classification.
type Target struct {
	BaseURL string     `yaml:"base_url"`
	Tier    model.Tier `yaml:"tier"`
	// OpenAPISpec, when set, is a spec the scenario's request is validated
	// against before any load is generated (IR-01, ADR-0008). Optional.
	OpenAPISpec string `yaml:"openapi_spec"`
}

// BlastRadius holds BlastGuard's mandatory ceilings and the auto-abort limits.
type BlastRadius struct {
	MaxConcurrency int       `yaml:"max_concurrency"`
	MaxDuration    Duration  `yaml:"max_duration"`
	AutoAbort      AutoAbort `yaml:"auto_abort"`
}

// AutoAbort configures the catastrophic-abort ceiling (SR-05). Both are
// optional; zero means "do not abort on this signal".
type AutoAbort struct {
	ErrorRateOver float64  `yaml:"error_rate_over"`
	P99Over       Duration `yaml:"p99_over"`
}

// Auth is optional. When present it supplies the login flow and credentials;
// the password commonly holds a ${VAR} reference expanded at load.
type Auth struct {
	LoginEndpoint string      `yaml:"login_endpoint"`
	Credentials   Credentials `yaml:"credentials"`
	TokenPath     string      `yaml:"token_path"`
}

// Credentials are the login fields.
type Credentials struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// Caps returns the blast-radius ceilings as the shared model type.
func (c *Config) Caps() model.Caps {
	return model.Caps{
		MaxConcurrency: c.BlastRadius.MaxConcurrency,
		MaxDuration:    time.Duration(c.BlastRadius.MaxDuration),
	}
}

// TargetEcho builds the config echo written into result.json. It copies only
// the non-secret target facts (base URL, tier, caps); auth is deliberately not
// carried, so there is no credential to redact and none can leak (SR-07).
func (c *Config) TargetEcho() model.TargetEcho {
	return model.TargetEcho{
		BaseURL: c.Target.BaseURL,
		Tier:    c.Target.Tier,
		Caps:    c.Caps(),
	}
}

// Duration is a time.Duration that unmarshals from a YAML duration string like
// "60s", since YAML has no native duration type.
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

// String renders the duration the "60s" way the config uses.
func (d Duration) String() string { return time.Duration(d).String() }

// Load reads path, expands ${VAR} references, parses the YAML, and validates
// every required field. Failures come back as a single error with a specific
// message — including, for missing required fields, all of them at once.
func Load(path string) (*Config, error) {
	// path is the operator's own --config argument, not untrusted input: whoever
	// runs sapper already has shell access to the same files, so reading the path
	// they name is the purpose, not a traversal (G304). os.Root does not fit — a
	// config legitimately lives anywhere, not under a fixed root.
	raw, err := os.ReadFile(path) // #nosec G304
	if err != nil {
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}

	if err := expandTree(&root); err != nil {
		return nil, fmt.Errorf("config: %s: %w", path, err)
	}

	var cfg Config
	// An empty file parses to a zero node, which Decode would reject. Leave cfg
	// at its zero value and let validation report what is missing.
	if root.Kind != 0 {
		if err := root.Decode(&cfg); err != nil {
			return nil, fmt.Errorf("config: parse %s: %w", path, err)
		}
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config: %s: %w", path, err)
	}
	return &cfg, nil
}

// expandTree rewrites ${VAR} references in the parsed document's scalar values,
// reporting every unset variable at once. Expansion happens on the parsed tree
// rather than the raw text so it cannot reach into comments: a file that
// documents its own "password: ${LAB_PASSWORD}" convention in a comment is
// describing the syntax, not asking for a variable to be resolved.
func expandTree(root *yaml.Node) error {
	missing := make(map[string]bool)
	var others []error

	var walk func(*yaml.Node)
	walk = func(n *yaml.Node) {
		if n.Kind == yaml.ScalarNode {
			expanded, err := envexpand.Expand(n.Value)
			if err != nil {
				var e *envexpand.MissingVarsError
				if errors.As(err, &e) {
					for _, name := range e.Names {
						missing[name] = true
					}
				} else {
					others = append(others, err)
				}
			} else {
				n.Value = expanded
			}
		}
		for _, child := range n.Content {
			walk(child)
		}
	}
	walk(root)

	if len(others) > 0 {
		return errors.Join(others...)
	}
	if len(missing) > 0 {
		return &envexpand.MissingVarsError{Names: slices.Sorted(maps.Keys(missing))}
	}
	return nil
}

func (c *Config) validate() error {
	var errs []error

	if c.SchemaVersion != SupportedSchemaVersion {
		errs = append(errs, fmt.Errorf("schema_version = %d, only %d is supported", c.SchemaVersion, SupportedSchemaVersion))
	}

	if c.Target.BaseURL == "" {
		errs = append(errs, errors.New("target.base_url is required"))
	} else if u, err := url.Parse(c.Target.BaseURL); err != nil || !u.IsAbs() || u.Host == "" {
		errs = append(errs, fmt.Errorf("target.base_url must be an absolute URL with a host, got %q", c.Target.BaseURL))
	}

	if c.Target.Tier == "" {
		errs = append(errs, errors.New("target.tier is required (lab|staging|authorized|production); there is no default (SR-01)"))
	} else if !c.Target.Tier.Valid() {
		errs = append(errs, fmt.Errorf("target.tier = %q is not a known tier (lab|staging|authorized|production) (SR-01)", c.Target.Tier))
	}

	if err := c.Caps().Validate(); err != nil {
		errs = append(errs, fmt.Errorf("blast_radius: %w", err))
	}

	return errors.Join(errs...)
}
