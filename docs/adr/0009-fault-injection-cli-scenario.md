# ADR-0009: Fault injection is a scenario profile, reusing Spike's shape and existing SLOs

## Status
Accepted

## Context
ADR-0004 reserved the `FaultInjector` port and deferred Case 4 — not just
building the injector, but driving it from a real `sapper run`. ADR-0007
settled how the injector itself works (a minimal in-process proxy) and
closed ADR-0004's reopening point (1). Its own point (2) — "revisit whether
the reserved port shape survived contact" — was still open: the injector
(`internal/adapters/inject`) existed and was proven inside one test
(`tiein_test.go`), driving `generator.RunSustained` directly, but nothing
let an operator reach it through `sapper run --scenario ... --config ...`
the way every other case does. `docs/CHANGELOG.md`'s own "Not included"
line named this gap by name.

## Decision
Fault injection is a fifth scenario `profile.type`: `fault-injection`. It
runs three phases — baseline, faulted, recovery — identical in shape to
Spike's own before/peak/after (`executeSpike`), except what changes between
phases is the dependency's health, not the load level, which stays constant
throughout. No new SLO type was added: `status_seen` (already built for
ramp-up's rate-limiter knee) asserts the breaker's own fast-fail status
appeared during the faulted phase, and `recovery_within` (already built for
spike) asserts the post-fault p99 returned close to the pre-fault one. The
scenario schema gains `upstream` (the real dependency), `listen` (the
address the injector binds to for the run), and `fault_kind`/`fault_status`/
`fault_delay` (what to inject). `cmd/sapper/run.go`'s `executeFaultInjection`
starts `inject.New` and a real `*http.Server` on `listen`, forwarding to
`upstream`, sharing the same `*blastguard.BlastGuard` the load path uses —
ADR-0007's own claim ("the same in-process guarantee the load path already
has") made concrete rather than merely asserted.

**The operational requirement this does not solve:** the target under test
must already be configured, outside Sapper, to send the one dependency's
traffic to `listen` instead of its real address, for the run's duration.
Sapper cannot reach into the target's own configuration to arrange this —
it was never going to, and ADR-0007 already said so ("the target is pointed
at the proxy's address for the run"). This ADR's own contribution is making
that requirement a named scenario field (`listen`) with a validation error
if it is missing, rather than an undocumented assumption.

## The alternative that was rejected
**A dedicated `sapper inject` subcommand**, separate from `run`, that only
starts the proxy and lets an operator drive load against the target with
a second, independent `sapper run` invocation (or any other tool) while it
runs. Rejected: it would require coordinating two processes' lifetimes by
hand, defeats `result.json`'s own single-file-per-run contract (there would
be two, needing to be correlated after the fact), and — the deciding
point — loses the three-phase shape that makes "did the breaker open AND
recover" a single, aggregated verdict rather than something stitched
together from two runs' own separate files.

## Consequences
- `scenario.Scenario` now carries five fields (`Upstream`, `Listen`,
  `FaultKind`, `FaultStatus`, `FaultDelay`) that mean nothing for every
  other profile type — the same cost Spike's own `BaselineConcurrency`/
  `BaselineDuration` and Soak's own `Windows` already accepted.
- The injector's own `ServeHTTP` (`internal/adapters/inject`) has never
  authenticated or restricted *who* may reach it — it forwards or faults
  based only on its own current `Fault` state. Exposing it as a real,
  reachable listener for the run's duration makes that pre-existing design
  choice externally visible for the first time; it was invisible while the
  only caller was a same-process test. The residual is the operator's own:
  `listen` should name an address only the target itself can reach
  (typically loopback, or an address inside the same private network as
  the target and nothing else) — Sapper validates that the field is
  non-empty, not that the address is appropriately scoped, the same way it
  validates `target.base_url` is absolute without judging whether it is
  wise to point Sapper at it.
- Found and fixed while writing this ADR's own implementation, not a design
  choice: `internal/adapters/inject`'s own test suite had no case proving
  the proxy stops injecting once `BlastGuard.Stop` fires — ADR-0007's own
  claim had never been exercised by a negative control. Added
  (`TestProxyStopsInjectingOnceGuardHalted`), seen failing with the
  `guard.Stopped()` check removed, before this ADR could honestly repeat
  the claim.

## Reopening criterion
A scenario needs the faulted phase's load to differ from the baseline's own
(the way Spike's peak differs from its baseline) — today they are
deliberately the same, since Case 4's own question is "does the dependency
failing change the outcome," not "does more load." If that changes, the
schema needs its own `Concurrency`/`BaselineConcurrency` pair distinguished
differently than Spike's, not reused as-is.

## Amendment target
This closes ADR-0004's reopening criterion point (2) in full: the reserved
port survived contact, driven now through the same `run`/`assert`/`report`
pipeline every other case uses, with no reshaping of the generator or
result model ADR-0004 itself predicted.
