// Package assert evaluates a run's aggregated metrics against its declared SLOs
// and produces a Verdict. It is a pure function of its inputs — no network, no
// clock — so CI can gate on it from a result file alone, and a report can be
// regenerated from an old result.
//
// Evaluation is variance-aware in the sense of ADR-0003: it asserts against the
// mean across N runs (less noisy than a single run) and surfaces the spread in
// each result's detail, so a metric that swings run to run is visible rather
// than hidden behind one lucky number.
package assert

import (
	"fmt"
	"time"

	"github.com/JonasBorgesLM/sapper/internal/core/metrics"
	"github.com/JonasBorgesLM/sapper/internal/core/model"
	"github.com/JonasBorgesLM/sapper/internal/core/scenario"
)

// Evaluate checks the aggregate against each declared SLO. The verdict passes
// only if every SLO passes.
func Evaluate(agg metrics.Aggregation, slos scenario.SLOs) model.Verdict {
	var results []model.SLOResult

	if slos.P99Under != nil {
		threshold := time.Duration(*slos.P99Under)
		results = append(results, model.SLOResult{
			Name:   "p99_under",
			Passed: agg.P99.Mean < threshold,
			Detail: fmt.Sprintf("p99 mean %s (±%s over %d runs) vs < %s", agg.P99.Mean, agg.P99.StdDev, agg.Runs, threshold),
		})
	}
	if slos.ErrorRateUnder != nil {
		threshold := *slos.ErrorRateUnder
		results = append(results, model.SLOResult{
			Name:   "error_rate_under",
			Passed: agg.ErrorRate.Mean < threshold,
			Detail: fmt.Sprintf("error rate mean %.4f (±%.4f over %d runs) vs < %.4f", agg.ErrorRate.Mean, agg.ErrorRate.StdDev, agg.Runs, threshold),
		})
	}
	if slos.StatusSeen != nil {
		code := *slos.StatusSeen
		count := agg.StatusCounts[code]
		results = append(results, model.SLOResult{
			Name:   "status_seen",
			Passed: count > 0,
			Detail: fmt.Sprintf("status %d observed %d times across %d runs (want ≥ 1 — e.g. the rate limiter's 429)", code, count, agg.Runs),
		})
	}

	verdict := model.Verdict{Passed: true, Results: results}
	for _, r := range results {
		if !r.Passed {
			verdict.Passed = false
		}
	}
	return verdict
}
