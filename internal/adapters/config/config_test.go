package config

import (
	"strings"
	"testing"
	"time"

	"github.com/JonasBorgesLM/sapper/internal/core/model"
)

func TestLoadValid(t *testing.T) {
	t.Setenv("SAPPER_TEST_PW", "s3cret")
	cfg, err := Load("testdata/valid.yaml")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.SchemaVersion != SupportedSchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", cfg.SchemaVersion, SupportedSchemaVersion)
	}
	if cfg.Target.BaseURL != "http://localhost:8080" {
		t.Errorf("BaseURL = %q", cfg.Target.BaseURL)
	}
	if cfg.Target.Tier != model.TierLab {
		t.Errorf("Tier = %q, want %q", cfg.Target.Tier, model.TierLab)
	}
	caps := cfg.Caps()
	if caps.MaxConcurrency != 50 || caps.MaxDuration != 60*time.Second {
		t.Errorf("Caps() = %+v, want {50, 60s}", caps)
	}
	if cfg.Auth.Credentials.Password != "s3cret" {
		t.Errorf("password = %q, want the expanded env value, not the literal ${VAR}", cfg.Auth.Credentials.Password)
	}
}

func TestLoadCommentPlaceholderIsNotExpanded(t *testing.T) {
	// valid.yaml's comment references ${SAPPER_TEST_UNSET_IN_COMMENT}, which is
	// never set. If expansion reached comments this would fail with a missing
	// variable; it must not.
	t.Setenv("SAPPER_TEST_PW", "s3cret")
	if _, err := Load("testdata/valid.yaml"); err != nil {
		t.Fatalf("Load() error = %v, want nil (a ${VAR} in a comment must be ignored)", err)
	}
}

func TestLoadRejectsMissingTier(t *testing.T) {
	_, err := Load("testdata/missing-tier.yaml")
	if err == nil || !strings.Contains(err.Error(), "tier") {
		t.Fatalf("Load() error = %v, want an error naming tier (SR-01)", err)
	}
}

func TestLoadRejectsUnknownTier(t *testing.T) {
	_, err := Load("testdata/unknown-tier.yaml")
	if err == nil || !strings.Contains(err.Error(), "tier") {
		t.Fatalf("Load() error = %v, want an error naming tier (SR-01)", err)
	}
}

func TestLoadRejectsMissingConcurrency(t *testing.T) {
	_, err := Load("testdata/missing-concurrency.yaml")
	if err == nil || !strings.Contains(err.Error(), "max_concurrency") {
		t.Fatalf("Load() error = %v, want an error naming max_concurrency (SR-03)", err)
	}
}

func TestLoadRejectsMissingDuration(t *testing.T) {
	_, err := Load("testdata/missing-duration.yaml")
	if err == nil || !strings.Contains(err.Error(), "max_duration") {
		t.Fatalf("Load() error = %v, want an error naming max_duration (SR-03)", err)
	}
}

func TestLoadRejectsMissingEnvVar(t *testing.T) {
	_, err := Load("testdata/missing-env-var.yaml")
	if err == nil || !strings.Contains(err.Error(), "SAPPER_TEST_DEFINITELY_UNSET_VAR") {
		t.Fatalf("Load() error = %v, want an error naming the unset variable", err)
	}
}

func TestLoadRejectsWrongSchemaVersion(t *testing.T) {
	_, err := Load("testdata/wrong-schema.yaml")
	if err == nil || !strings.Contains(err.Error(), "schema_version") {
		t.Fatalf("Load() error = %v, want an error naming schema_version", err)
	}
}

func TestLoadRejectsRelativeBaseURL(t *testing.T) {
	_, err := Load("testdata/relative-base-url.yaml")
	if err == nil || !strings.Contains(err.Error(), "base_url") {
		t.Fatalf("Load() error = %v, want an error naming base_url", err)
	}
}
