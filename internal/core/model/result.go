package model

import (
	"time"

	"github.com/JonasBorgesLM/sapper/internal/core/metrics"
)

// Result is the on-disk shape of result.json: the single artifact `sapper run`
// writes and `sapper assert`/`sapper report` read. This is the stable envelope
// — the config echo and run outcome that every profile produces. The metrics,
// declared SLOs and verdict payloads are attached by the issues that produce
// them (the collector and the assertion stage); they are added to this struct,
// never replacing what is here.
//
// The config echo carries no credentials by construction (SR-07): TargetEcho
// has no password field, so a committed result.json cannot relocate a secret.
type Result struct {
	Target      TargetEcho `json:"target"`
	Scenario    string     `json:"scenario"`
	Profile     string     `json:"profile"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt time.Time  `json:"completed_at,omitempty"`
	// Aborted is true when BlastGuard stopped the run before it finished
	// (a cap, the kill switch, or the auto-abort ceiling). AbortReason names
	// which — an aborted run is honest data, not a failure to hide.
	Aborted     bool   `json:"aborted"`
	AbortReason string `json:"abort_reason,omitempty"`

	// Metrics is the aggregate across the run's N repetitions; Verdict is the
	// SLO evaluation. These attach to the envelope (they are produced by the
	// collector and the assert stage), and are what `report` and `assert` read.
	Metrics metrics.Aggregation `json:"metrics"`
	Verdict Verdict             `json:"verdict"`
}

// TargetEcho records what was targeted and under which ceilings, so a result is
// self-describing. It deliberately omits auth: there is nothing here to redact
// because credentials never enter the struct.
type TargetEcho struct {
	BaseURL string `json:"base_url"`
	Tier    Tier   `json:"tier"`
	Caps    Caps   `json:"caps"`
}
