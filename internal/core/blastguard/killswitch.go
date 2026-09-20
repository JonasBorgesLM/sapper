package blastguard

import "context"

// WatchContext halts g when ctx is cancelled, recording reason. It blocks until
// the context is done, so callers run it in a goroutine for the run's duration.
//
// In cmd/sapper the context comes from signal.NotifyContext(SIGINT, SIGTERM),
// which makes Ctrl+C the kill switch (SR-04): Stop denies new load immediately,
// while requests already in flight drain to completion (release simply frees
// their slots) — an immediate halt of new load with a graceful shutdown, not a
// mid-request kill.
func WatchContext(ctx context.Context, g *BlastGuard, reason string) {
	<-ctx.Done()
	g.Stop(reason)
}
