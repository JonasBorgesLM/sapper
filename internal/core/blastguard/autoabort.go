package blastguard

import (
	"context"
	"fmt"
	"time"
)

// AutoAbortLimits are the catastrophic-abort ceilings (SR-05): a transport error
// rate and a p99 latency above which continuing is useless data and gratuitous
// damage. A zero field means that signal is disabled — there is no accidental
// "abort at 0", only an explicit ceiling.
type AutoAbortLimits struct {
	ErrorRateOver float64
	P99Over       time.Duration
}

// Exceeded reports whether the observed error rate or p99 has crossed a
// configured ceiling, with a reason naming which. A zero ceiling is skipped.
func (l AutoAbortLimits) Exceeded(errorRate float64, p99 time.Duration) (reason string, ok bool) {
	if l.ErrorRateOver > 0 && errorRate > l.ErrorRateOver {
		return fmt.Sprintf("error rate %.1f%% over the %.1f%% ceiling", errorRate*100, l.ErrorRateOver*100), true
	}
	if l.P99Over > 0 && p99 > l.P99Over {
		return fmt.Sprintf("p99 %s over the %s ceiling", p99, l.P99Over), true
	}
	return "", false
}

// WatchAutoAbort samples the live stats every interval and halts the guard the
// first time a ceiling is crossed, recording why (SR-05). It returns when it
// stops the guard, when the guard is already stopped, or when ctx is cancelled,
// so callers run it in a goroutine for the run's duration. sample returns the
// current transport error rate and p99 (wired to the collector by the
// generator); decoupling it this way keeps the watcher unit-testable.
func WatchAutoAbort(ctx context.Context, g *BlastGuard, sample func() (errorRate float64, p99 time.Duration), limits AutoAbortLimits, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if g.Stopped() {
				return
			}
			if reason, ok := limits.Exceeded(sample()); ok {
				g.Stop("auto-abort: " + reason)
				return
			}
		}
	}
}
