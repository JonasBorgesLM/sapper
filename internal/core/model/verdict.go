package model

// Verdict is the green/red outcome of asserting a run's metrics against its
// declared SLOs — the primary output of `sapper assert` and the reason Sapper
// is a correctness tool, not a benchmark (ADR-0001). It is written into
// result.json and drives the CLI exit code.
type Verdict struct {
	Passed  bool        `json:"passed"`
	Results []SLOResult `json:"results"`
}

// SLOResult is the outcome of one declared SLO, with a human-readable detail
// that names the observed value, its spread across runs, and the threshold.
type SLOResult struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail"`
}
