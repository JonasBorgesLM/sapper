// Package generator is the load engine: it drives a load profile, emitting every
// request through the guarded client (so BlastGuard admits each one) and
// recording the outcome in the collector. v1 implements the sustained profile;
// ramp-up, spike and soak reuse this engine.
package generator

import (
	"context"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/JonasBorgesLM/sapper/internal/core/blastguard"
	"github.com/JonasBorgesLM/sapper/internal/core/metrics"
	"github.com/JonasBorgesLM/sapper/internal/ports"
)

// RequestFunc builds a fresh request for each send. It takes the context so a
// request is cancelled when the run ends.
type RequestFunc func(ctx context.Context) (*http.Request, error)

// Sustained is the sustained profile's parameters: a fixed number of concurrent
// workers driving load for Duration, after an optional Warmup whose samples are
// discarded.
type Sustained struct {
	Concurrency int
	Duration    time.Duration
	Warmup      time.Duration
}

// RunSustained runs the sustained profile to completion. It launches Concurrency
// workers that repeatedly send through client and record into coll, for
// Warmup+Duration; at the warmup mark it resets the collector so warmup samples
// are excluded (FR-07). It returns when the window elapses, when ctx is
// cancelled, or when the guard halts (kill switch, cap, or auto-abort).
func RunSustained(ctx context.Context, client ports.Requester, coll *metrics.Collector, newReq RequestFunc, p Sustained) error {
	ctx, cancel := context.WithTimeout(ctx, p.Warmup+p.Duration)
	defer cancel()

	if p.Warmup > 0 {
		go func() {
			timer := time.NewTimer(p.Warmup)
			defer timer.Stop()
			select {
			case <-timer.C:
				coll.Reset()
			case <-ctx.Done():
			}
		}()
	}

	var wg sync.WaitGroup
	for i := 0; i < p.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			worker(ctx, client, coll, newReq)
		}()
	}
	wg.Wait()
	return nil
}

func worker(ctx context.Context, client ports.Requester, coll *metrics.Collector, newReq RequestFunc) {
	for {
		if ctx.Err() != nil {
			return
		}
		req, err := newReq(ctx)
		if err != nil {
			coll.Record(0, 0, err)
			continue
		}
		start := time.Now()
		resp, err := client.Do(req)
		latency := time.Since(start)
		if err != nil {
			// A guard halt, or a request cancelled because the run's own window
			// ended, is not a target failure — stop the worker without recording
			// it, so an aborted or completed run's metrics reflect real requests.
			if errors.Is(err, blastguard.ErrAborted) ||
				errors.Is(err, context.Canceled) ||
				errors.Is(err, context.DeadlineExceeded) {
				return
			}
			coll.Record(latency, 0, err)
			continue
		}
		coll.Record(latency, resp.StatusCode, nil)
		// Drain and close so the connection is reused rather than leaked under
		// sustained load.
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}
}

// RampUp is the ramp-up profile: concurrency climbs from ~0 to MaxConcurrency
// over Duration, so the load crosses a rate limiter's knee and reveals where it
// engages.
type RampUp struct {
	MaxConcurrency int
	Duration       time.Duration
}

// RunRampUp runs the ramp-up profile. It staggers each worker's start evenly
// across Duration, so active concurrency rises linearly to MaxConcurrency, then
// the window closes. Workers share the same guarded client and collector as the
// sustained profile.
func RunRampUp(ctx context.Context, client ports.Requester, coll *metrics.Collector, newReq RequestFunc, p RampUp) error {
	ctx, cancel := context.WithTimeout(ctx, p.Duration)
	defer cancel()

	var wg sync.WaitGroup
	for i := 0; i < p.MaxConcurrency; i++ {
		delay := time.Duration(int64(p.Duration) * int64(i) / int64(p.MaxConcurrency))
		wg.Add(1)
		go func(delay time.Duration) {
			defer wg.Done()
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return
			}
			worker(ctx, client, coll, newReq)
		}(delay)
	}
	wg.Wait()
	return nil
}
