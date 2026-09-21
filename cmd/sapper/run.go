package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/JonasBorgesLM/sapper/internal/adapters/config"
	"github.com/JonasBorgesLM/sapper/internal/adapters/httpclient"
	"github.com/JonasBorgesLM/sapper/internal/core/assert"
	"github.com/JonasBorgesLM/sapper/internal/core/blastguard"
	"github.com/JonasBorgesLM/sapper/internal/core/generator"
	"github.com/JonasBorgesLM/sapper/internal/core/metrics"
	"github.com/JonasBorgesLM/sapper/internal/core/model"
	"github.com/JonasBorgesLM/sapper/internal/core/scenario"
)

// runRun implements `sapper run`: it executes a scenario against the configured
// target and writes a single result.json.
func runRun(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	configPath := fs.String("config", "", "path to the safety config (target, tier, blast-radius caps)")
	scenarioPath := fs.String("scenario", "", "path to the scenario (load profile + SLOs)")
	out := fs.String("out", "result.json", "path to write the result JSON")
	runs := fs.Int("runs", 3, "number of repetitions to aggregate (statistical honesty; ADR-0003)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *configPath == "" || *scenarioPath == "" {
		return fmt.Errorf("both --config and --scenario are required")
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	sc, err := scenario.Load(*scenarioPath)
	if err != nil {
		return err
	}

	result, err := executeRun(ctx, cfg, sc, *runs, interactiveConfirm)
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding result: %w", err)
	}
	// result.json carries no secrets (SR-07 redacts them) and is meant to be
	// read, shared and committed, so 0644 is appropriate rather than gosec's
	// conservative 0600 default.
	if err := os.WriteFile(*out, data, 0o644); err != nil { // #nosec G306
		return fmt.Errorf("writing %s: %w", *out, err)
	}

	printRunSummary(result, *out)
	return nil
}

// executeRun builds the guard, client and generator, runs the scenario `runs`
// times, aggregates the repetitions, asserts the SLOs, and assembles the result.
// It is the critical flow and is exercised end to end against a real server in
// the tests. confirm is used only for a production tier (SR-02).
func executeRun(ctx context.Context, cfg *config.Config, sc *scenario.Scenario, runs int, confirm func() (bool, error)) (model.Result, error) {
	start := time.Now()

	var opts []blastguard.Option
	if cfg.Target.Tier.IsProduction() {
		opts = append(opts, blastguard.WithProductionApproval(confirm))
	}
	guard, err := blastguard.New(cfg.Target.Tier, cfg.Caps(), opts...)
	if err != nil {
		return model.Result{}, err
	}

	// The kill switch: cancelling ctx (SIGINT/SIGTERM, wired in main) halts the
	// guard, which stops new load while in-flight requests drain (SR-04).
	go blastguard.WatchContext(ctx, guard, "shutdown signal received")

	client := httpclient.New(guard, nil)
	newReq := func(rctx context.Context) (*http.Request, error) {
		return http.NewRequestWithContext(rctx, http.MethodGet, cfg.Target.BaseURL, nil)
	}
	limits := blastguard.AutoAbortLimits{
		ErrorRateOver: cfg.BlastRadius.AutoAbort.ErrorRateOver,
		P99Over:       time.Duration(cfg.BlastRadius.AutoAbort.P99Over),
	}

	if sc.Profile.Type == scenario.ProfileSpike {
		return executeSpike(ctx, cfg, sc, guard, client, newReq, limits, start), nil
	}

	if runs < 1 {
		runs = 1
	}
	snaps := make([]metrics.Snapshot, 0, runs)
	for i := 0; i < runs; i++ {
		if guard.Stopped() {
			break // a catastrophic abort or kill switch halts every remaining run
		}
		coll := metrics.New()
		runCtx, cancelRun := context.WithCancel(ctx)
		go blastguard.WatchAutoAbort(runCtx, guard, func() (float64, time.Duration) {
			s := coll.Snapshot()
			return s.ErrorRate, s.Latency.P99
		}, limits, time.Second)
		runProfile(runCtx, sc, client, coll, newReq)
		cancelRun()
		snaps = append(snaps, coll.Snapshot())
	}

	agg := metrics.Aggregate(snaps)
	return model.Result{
		Target:      cfg.TargetEcho(),
		Scenario:    sc.Name,
		Profile:     sc.Profile.Type,
		StartedAt:   start,
		CompletedAt: time.Now(),
		Aborted:     guard.Stopped(),
		AbortReason: guard.Reason(),
		Metrics:     agg,
		Verdict:     assert.Evaluate(agg, sc.SLOs),
	}, nil
}

// executeSpike runs the spike profile: a baseline phase, a peak, then a
// baseline phase again. The recovery SLO compares the after-baseline p99 to the
// before-baseline p99, so the run answers "did latency return to normal after
// the spike?" rather than a fixed threshold. Metrics report the aggregate of
// all three phases; the verdict combines any generic SLOs with the recovery.
func executeSpike(ctx context.Context, cfg *config.Config, sc *scenario.Scenario, guard *blastguard.BlastGuard, client *httpclient.Client, newReq generator.RequestFunc, limits blastguard.AutoAbortLimits, start time.Time) model.Result {
	baseline := generator.Sustained{Concurrency: sc.Profile.BaselineConcurrency, Duration: time.Duration(sc.Profile.BaselineDuration)}
	peak := generator.Sustained{Concurrency: sc.Profile.Concurrency, Duration: time.Duration(sc.Profile.Duration)}

	runPhase := func(p generator.Sustained) metrics.Snapshot {
		if guard.Stopped() {
			return metrics.Snapshot{}
		}
		coll := metrics.New()
		phaseCtx, cancel := context.WithCancel(ctx)
		go blastguard.WatchAutoAbort(phaseCtx, guard, func() (float64, time.Duration) {
			s := coll.Snapshot()
			return s.ErrorRate, s.Latency.P99
		}, limits, time.Second)
		_ = generator.RunSustained(phaseCtx, client, coll, newReq, p)
		cancel()
		return coll.Snapshot()
	}

	before := runPhase(baseline)
	spikeSnap := runPhase(peak)
	after := runPhase(baseline)

	agg := metrics.Aggregate([]metrics.Snapshot{before, spikeSnap, after})
	verdict := assert.Evaluate(agg, sc.SLOs)
	if sc.SLOs.RecoveryWithin != nil {
		rec := assert.Recovery(before, after, *sc.SLOs.RecoveryWithin)
		verdict.Results = append(verdict.Results, rec)
		if !rec.Passed {
			verdict.Passed = false
		}
	}

	return model.Result{
		Target:      cfg.TargetEcho(),
		Scenario:    sc.Name,
		Profile:     sc.Profile.Type,
		StartedAt:   start,
		CompletedAt: time.Now(),
		Aborted:     guard.Stopped(),
		AbortReason: guard.Reason(),
		Metrics:     agg,
		Verdict:     verdict,
	}
}

// runProfile drives one repetition of the scenario's load profile through the
// guarded client into coll. New profiles (spike, soak) add a case here.
func runProfile(ctx context.Context, sc *scenario.Scenario, client *httpclient.Client, coll *metrics.Collector, newReq generator.RequestFunc) {
	switch sc.Profile.Type {
	case scenario.ProfileRampUp:
		_ = generator.RunRampUp(ctx, client, coll, newReq, generator.RampUp{
			MaxConcurrency: sc.Profile.Concurrency,
			Duration:       time.Duration(sc.Profile.Duration),
		})
	default: // sustained
		_ = generator.RunSustained(ctx, client, coll, newReq, generator.Sustained{
			Concurrency: sc.Profile.Concurrency,
			Duration:    time.Duration(sc.Profile.Duration),
			Warmup:      time.Duration(sc.Profile.Warmup),
		})
	}
}

// verdictExitCode maps a verdict to a process exit code so CI can gate on it.
func verdictExitCode(v model.Verdict) int {
	if v.Passed {
		return 0
	}
	return 1
}

// interactiveConfirm prompts on stderr and reads a yes/no from stdin. In a
// non-interactive context (CI) the read fails and it declines, so production is
// refused there (SR-02).
func interactiveConfirm() (bool, error) {
	fmt.Fprint(os.Stderr, "About to generate load against a PRODUCTION target. Type 'yes' to continue: ")
	var resp string
	if _, err := fmt.Fscanln(os.Stdin, &resp); err != nil {
		return false, nil
	}
	return resp == "yes", nil
}

func printRunSummary(r model.Result, out string) {
	fmt.Printf("run %q (%s) → %s\n", r.Scenario, r.Profile, out)
	fmt.Printf("  p99 mean %s (±%s over %d runs), error rate %.4f\n",
		r.Metrics.P99.Mean, r.Metrics.P99.StdDev, r.Metrics.Runs, r.Metrics.ErrorRate.Mean)
	if r.Aborted {
		fmt.Printf("  ABORTED: %s\n", r.AbortReason)
	}
	verdict := "PASS"
	if !r.Verdict.Passed {
		verdict = "FAIL"
	}
	fmt.Printf("  verdict: %s (run `sapper assert --in %s` to gate)\n", verdict, out)
}
