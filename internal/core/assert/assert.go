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

	if slos.StatusAbsent != nil {
		results = append(results, statusAbsent(agg, *slos.StatusAbsent))
	}
	if slos.StatusFractionUnder != nil {
		results = append(results, statusFractionUnder(agg, *slos.StatusFractionUnder))
	}

	verdict := model.Verdict{Passed: true, Results: results}
	for _, r := range results {
		if !r.Passed {
			verdict.Passed = false
		}
	}
	return verdict
}

// responses is the number of requests that got an HTTP response; the
// denominator of the status SLOs, which say nothing about transport failures.
func responses(agg metrics.Aggregation) int {
	n := 0
	for _, c := range agg.StatusCounts {
		n += c
	}
	return n
}

// statusAbsent: no response answered code. With no responses at all it fails —
// "no 500 seen" is vacuously true of a run that never reached the target.
func statusAbsent(agg metrics.Aggregation, code int) model.SLOResult {
	total, seen := responses(agg), agg.StatusCounts[code]
	return model.SLOResult{
		Name:   "status_absent",
		Passed: total > 0 && seen == 0,
		Detail: fmt.Sprintf("status %d observed %d times in %d responses across %d runs (want 0)", code, seen, total, agg.Runs),
	}
}

// statusFractionUnder: responses answering f.Status are strictly under
// f.Fraction of all responses. It fails with no responses, as statusAbsent does.
func statusFractionUnder(agg metrics.Aggregation, f scenario.StatusFraction) model.SLOResult {
	total, seen := responses(agg), agg.StatusCounts[f.Status]
	fraction := 0.0
	if total > 0 {
		fraction = float64(seen) / float64(total)
	}
	return model.SLOResult{
		Name:   "status_fraction_under",
		Passed: total > 0 && fraction < f.Fraction,
		Detail: fmt.Sprintf("status %d was %d of %d responses (%.2f%%) across %d runs vs < %.2f%%", f.Status, seen, total, fraction*100, agg.Runs, f.Fraction*100),
	}
}

// Recovery evaluates the spike profile's recovery SLO: after the spike, the p99
// must return to within factor× the pre-spike baseline p99. It is separate from
// Evaluate because it compares two phases rather than an aggregate against a
// fixed threshold.
func Recovery(before, after metrics.Snapshot, factor float64) model.SLOResult {
	threshold := time.Duration(float64(before.Latency.P99) * factor)
	return model.SLOResult{
		Name:   "recovery_within",
		Passed: after.Latency.P99 <= threshold,
		Detail: fmt.Sprintf("post-spike p99 %s vs baseline %s × %.2f = %s", after.Latency.P99, before.Latency.P99, factor, threshold),
	}
}

// RecoveredStatus evaluates the fault-injection profile's recovered_status
// SLO (#57): at least rs.MinFraction of the after-phase's requests answered
// rs.Status. Unlike Recovery, which compares latency and so passes for a
// breaker stuck open (its local refusals are fast), this asks whether the
// target is actually serving again. An after-phase with no requests fails:
// a check that could not run has not passed.
func RecoveredStatus(after metrics.Snapshot, rs scenario.RecoveredStatus) model.SLOResult {
	seen := after.StatusCounts[rs.Status]
	fraction := 0.0
	if after.Total > 0 {
		fraction = float64(seen) / float64(after.Total)
	}
	return model.SLOResult{
		Name:   "recovered_status",
		Passed: after.Total > 0 && fraction >= rs.MinFraction,
		Detail: fmt.Sprintf("after the fault, %d of %d requests (%.1f%% by request, not time) answered %d; want ≥ %.1f%%", seen, after.Total, fraction*100, rs.Status, rs.MinFraction*100),
	}
}

// Degradation evaluates the soak profile's SLO: the last measurement window's
// p99 must be within factor× the first window's, so slow degradation over a long
// run (a leak, a growing queue) is caught. Like Recovery it compares two phases.
func Degradation(first, last metrics.Snapshot, factor float64) model.SLOResult {
	threshold := time.Duration(float64(first.Latency.P99) * factor)
	return model.SLOResult{
		Name:   "degradation_under",
		Passed: last.Latency.P99 <= threshold,
		Detail: fmt.Sprintf("final-window p99 %s vs first-window %s × %.2f = %s", last.Latency.P99, first.Latency.P99, factor, threshold),
	}
}

// WithAbort folds a run's aborted state into its verdict: a run halted by the
// kill switch, an auto-abort, or the duration cap did not complete, so its SLOs
// were evaluated on partial data and the verdict must not be green (H1). A
// completed run is returned unchanged.
func WithAbort(v model.Verdict, aborted bool, reason string) model.Verdict {
	if !aborted {
		return v
	}
	v.Passed = false
	v.Results = append(v.Results, model.SLOResult{
		Name:   "run_completed",
		Passed: false,
		Detail: "run aborted before completion: " + reason,
	})
	return v
}
