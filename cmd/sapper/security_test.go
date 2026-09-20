package main

import (
	"context"
	"testing"
	"time"

	"github.com/JonasBorgesLM/sapper/internal/adapters/config"
	"github.com/JonasBorgesLM/sapper/internal/core/model"
	"github.com/JonasBorgesLM/sapper/internal/core/scenario"
)

// This file is the integration-level security suite: it proves executeRun — the
// composition that assembles the guard, client and generator — refuses to
// generate load unless it is safe to. Each unit protection is also tested in its
// own package; these tests lock in the property at the boundary an operator
// actually uses. Every one has been verified by a negative control (removing the
// protection turns it red).
//
//	SR-01  unknown/empty tier refused   → TestExecuteRunRefusesUnknownTier
//	SR-02  production needs confirmation → TestExecuteRunRefusesProduction*
//	SR-03  caps required/positive        → blastguard caps_test + config_test
//	SR-06  no load bypasses the guard    → httpclient TestDoBlockedRequest...

func secScenario() *scenario.Scenario {
	p99 := scenario.Duration(time.Second)
	return &scenario.Scenario{
		Name:    "sec",
		Profile: scenario.Profile{Type: "sustained", Concurrency: 2, Duration: scenario.Duration(20 * time.Millisecond)},
		SLOs:    scenario.SLOs{P99Under: &p99},
	}
}

func secConfig(tier model.Tier) *config.Config {
	return &config.Config{
		SchemaVersion: 1,
		// A base URL that must never be dialled if the guard refuses first.
		Target:      config.Target{BaseURL: "http://127.0.0.1:9", Tier: tier},
		BlastRadius: config.BlastRadius{MaxConcurrency: 2, MaxDuration: config.Duration(time.Minute)},
	}
}

func TestExecuteRunRefusesUnknownTier(t *testing.T) {
	_, err := executeRun(context.Background(), secConfig("prod"), secScenario(), 1, nil)
	if err == nil {
		t.Fatal("executeRun ran against an unknown tier (SR-01)")
	}
}

func TestExecuteRunRefusesProductionWithoutConfirmation(t *testing.T) {
	// nil confirm models a non-interactive context (CI).
	_, err := executeRun(context.Background(), secConfig(model.TierProduction), secScenario(), 1, nil)
	if err == nil {
		t.Fatal("executeRun ran against production non-interactively (SR-02)")
	}
}

func TestExecuteRunRefusesProductionWhenConfirmationDeclined(t *testing.T) {
	declined := func() (bool, error) { return false, nil }
	_, err := executeRun(context.Background(), secConfig(model.TierProduction), secScenario(), 1, declined)
	if err == nil {
		t.Fatal("executeRun ran against production after the confirmation was declined (SR-02)")
	}
}
