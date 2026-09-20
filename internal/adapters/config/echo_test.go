package config

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/JonasBorgesLM/sapper/internal/core/model"
)

// The config echo written into result.json must never carry a secret (SR-07,
// invariant #4): result.json is committed in examples and CI, so a leaked
// credential there relocates the secret rather than reporting anything.
func TestTargetEchoExcludesSecrets(t *testing.T) {
	const secret = "s3cr3t-value-do-not-leak"
	t.Setenv("SAPPER_TEST_PW", secret)

	cfg, err := Load("testdata/valid.yaml")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Auth.Credentials.Password != secret {
		t.Fatalf("precondition: password not loaded, got %q", cfg.Auth.Credentials.Password)
	}

	echo := cfg.TargetEcho()
	b, err := json.Marshal(echo)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if strings.Contains(string(b), secret) {
		t.Errorf("target echo leaked the secret (SR-07): %s", b)
	}
	if echo.BaseURL != "http://localhost:8080" || echo.Tier != model.TierLab {
		t.Errorf("echo dropped non-secret fields: %+v", echo)
	}
}
