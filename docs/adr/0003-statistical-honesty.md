# ADR-0003: Statistical honesty replaces byte-for-byte determinism

## Status
Accepted

## Context
Warden holds byte-for-byte determinism as an invariant: same input, same output.
Sapper cannot — p99 latency varies run to run by nature. Dropping the invariant
without replacing it invites the classic load-tool dishonesty: run it a few
times, report the best number, call the defense validated.

## Decision
Replace determinism with **statistical honesty**, mechanically enforced:
- A **warm-up** window whose samples are excluded from reported statistics.
- **N runs**, aggregated — never a single run, never "the best run."
- Report **percentiles + variance**, not a point estimate.
- SLOs on a percentile are evaluated **against the reported variance**, so a gate
  does not flap on noise.

Structure stays deterministic (NFR-04): same config → same requests constructed,
same SLOs evaluated; only measured latencies vary.

## The alternative that was rejected
**Report a single run's percentiles** (simpler, one pass). Rejected: a single run
hides variance, and variance is exactly the signal that separates "the defense
holds" from "the defense held once." A p99 that is 40ms one run and 900ms the next
is not a 40ms system, and a tool that reports 40ms is lying.

## Consequences
- Runs take longer (N × plus warm-up) and the report is busier (distributions, not
  numbers). Accepted — it is the honest shape.
- The metrics collector must be cheap enough not to distort what it measures
  (FR-06); the histogram is the most likely place an external dependency is
  justified, and choosing one gets its own ADR.

## Reopening criterion
Choosing a specific histogram library/dependency — record it in a new ADR at that
point (see index open questions).
