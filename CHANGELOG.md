# Changelog

Notable changes to Sapper. Format based on Keep a Changelog; the project adopts
semantic versioning once it is first tagged.

## [0.1.0] — unreleased

First working version. Sapper applies sustained adversarial load, and controlled
fault injection, to an authorized target and asserts the observed behaviour
against SLOs declared in the scenario — a green/red verdict, gateable in CI.

### Added
- **Load profiles** — sustained, ramp-up, spike, and soak, driven by a shared
  engine through the guarded client into the metrics collector.
- **SLO assertions** — `p99_under`, `error_rate_under`, `status_seen`,
  `recovery_within` (spike), `degradation_under` (soak); evaluated variance-aware
  across N runs, never a single best run (ADR-0003).
- **BlastGuard safety layer** (SR-01..SR-06) — environment tier gate plus an
  interactive production gate a non-interactive CI cannot clear, mandatory
  concurrency and duration caps, a kill switch, catastrophic auto-abort, and a
  single guarded HTTP client with no unguarded constructor (ADR-0002).
- **Metrics** — exact latency percentiles (nearest-rank), per-status counts,
  transport error rate, warm-up discard, and N-run aggregation with spread.
- **Fault injection** — an in-process reverse proxy injecting error / latency /
  dropped-connection faults, behind the tier gate (ADR-0007); a tie-in test
  proves a circuit breaker opens and recovers under an injected fault.
- **OpenAPI import** — resolve and validate a scenario's target endpoint against
  a spec, parsed with the YAML library rather than a heavy dependency (ADR-0008).
- **CLI** — `run`, `assert`, `report`: independent stages chained through
  result.json, with a self-contained HTML report and a CI-gating exit code.
- **Config** — YAML with `${ENV}` secrets, required tier and blast-radius caps,
  and secret redaction in the result (SR-07).
- **Tooling** — example config and scenarios, and a CI pipeline with build,
  lint, gosec, govulncheck, and documentation and ADR-immutability guards.

### Decisions
Recorded as ADRs 0001–0008 in docs/adr/.

### Not included
- A CLI-driven chaos scenario; the fault injector is currently exercised through
  the bastion tie-in test, not a `run` profile.
- `$ref` indirection or schema-derived request bodies from OpenAPI specs — the
  reopening trigger noted in ADR-0008.
