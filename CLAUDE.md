# CLAUDE.md

Guidance for Claude Code in this repository.

The general engineering rules — effort proportional to the task, architecture
discipline, clean code, testing, review, security, git hygiene, verification —
live in `~/.claude/CLAUDE.md` and are already loaded. This file carries only what
is true of Sapper.

**This project is pre-code.** What exists today is the design: `README.md`,
`docs/REQUIREMENTS.md`, `docs/THREAT-MODEL.md`, `docs/architecture.md`, and
`docs/adr/`. Read the architecture and the threat model before writing the first
line of implementation — the safety properties are structural, not additive, and
retrofitting them is how they end up bypassable.

## Project Overview

Sapper is a Go CLI resilience test harness: it applies sustained adversarial load
plus controlled fault injection to an API and **asserts the result against a
declared expectation** (SLOs), green or red, gateable in CI. It is the offensive
complement to **Warden**, the black-box scanner (repo: `security-scanner`) —
Warden inspects, Sapper mines. See [ADR-0005](docs/adr/0005-warden-sapper-naming.md).

It is built for study, targeting **only the author's own lab/staging APIs**.

**Deliberately not responsible for:**
- **Throughput measurement as an end in itself** — that is k6/Locust. Sapper
  always asserts; a run with no SLO is a misconfiguration, not a benchmark. See
  [ADR-0001](docs/adr/0001-assertion-not-benchmark.md).
- **Fuzzing** — exploration of malformed input is a different question (input, not
  load/time) and a different tool.
- **Running against production without the explicit noisy gate.** See
  BlastGuard, below.

## Architecture

Lightweight hexagonal (ports/adapters), mirroring Warden so checks/scenarios can
be tested against fakes with no real network. Full detail in
[`docs/architecture.md`](docs/architecture.md).

- The **load generator** is the core. Every unit of load passes through
  **BlastGuard**, wired as mandatory middleware — the structural mirror of
  Warden's `ScopeGuard`. Nothing generates load without going through it. This is
  the single most important invariant in the codebase.
- The **fault-injector port** is defined but **not implemented in v1** (Case 4 is
  deferred). Reserve the seam; do not build behind it until the load spine is
  proven. See [ADR-0004](docs/adr/0004-defer-fault-injector.md).
- Subcommands are independent pipeline stages chained through JSON on disk
  (`run → assert → report`), the same philosophy as Warden.

## Stack

- **Go `1.25.0` floor** (matches `security-scanner` and `bastion`). The `go`
  directive is the minimum *language* version; pin the build `toolchain` in
  `go.mod` for the reason documented in Warden's `go.mod` — `golangci-lint`/
  `staticcheck` fail with `export data version ...` errors that look like stdlib
  bugs when the local Go is newer than the tool's build.
- Module path: `github.com/JonasBorgesLM/sapper`.
- Standard-library-first, like the siblings. Any external dependency needs a
  justification recorded in an ADR (notably: the metrics histogram — see
  [ADR-0003](docs/adr/0003-statistical-honesty.md) — must be cheap enough not to
  distort the measurement it takes).

## Commands

**Planned — no code exists yet.** These are the intended shapes, to be verified
against the real binary once it exists:

```bash
go build ./...
go vet ./...
GOTOOLCHAIN=go1.25.x golangci-lint run ./...   # pin the toolchain; see Stack
go test ./...
go build -o sapper ./cmd/sapper

sapper run    --scenario scenarios/sustained.yaml --config config.yaml --out result.json
sapper assert --in result.json                 # exit non-zero when an SLO is red (CI gate)
sapper report --in result.json --out report.html
```

## Conventions

- **Config is YAML + `${ENV}`** for secrets, with a `schema_version`, mirroring
  Warden. The environment tier and both blast-radius caps (max concurrency, max
  duration) are **required fields** — the loader must fail loudly when they are
  absent, never apply a default.
- **Stage output is JSON on disk.** A stage reads the previous stage's file; no
  hidden shared state between subcommands.
- **Cross-project requirement citations are written qualified** — `bastion/…`,
  `warden/…` — so they are never silently checked against Sapper's own numbering.

## Testing

- The load generator and BlastGuard must be unit-testable without a real target:
  drive them against a fake clock and a fake HTTP client, the way Warden tests
  checks against a fake `HTTPClient`.
- **Every BlastGuard property is a security invariant and must be seen failing
  before it is trusted** (global rule): remove the guard, watch the test go red,
  put it back. A guard whose test has never been red is hoped, not verified.
- **Assert the expected answer, not the absence of a wrong one.** A negative
  assertion ("no request exceeded the cap") is satisfied by a generator that never
  ran. Assert that load *was* generated *and* stayed within the cap.

## Known Constraints

- **No byte-for-byte determinism.** p99 varies run to run; that determinism is
  impossible here and is replaced by *statistical honesty* — N runs, warm-up
  discarded, percentiles + variance reported, never "the best run." See
  [ADR-0003](docs/adr/0003-statistical-honesty.md). Any assertion on a percentile
  must account for variance, or it is a coin flip dressed as a gate.
- **HTTP(S) targets only** in the near term. gRPC and other protocols are out of
  scope until a requirement asks for them.

## Non-negotiable Invariants

1. **No load bypasses BlastGuard.** If a code path can emit a request to the
   target without passing through the guard, that is a defect of the highest
   severity, regardless of what it enables. The guard is the difference between
   this tool and a DoS weapon. (`docs/THREAT-MODEL.md` T-01, T-02, T-03.)
2. **Both blast-radius caps are required and enforced before the first request.**
   Max concurrency and max duration are validated at config load; a missing or
   non-positive value aborts startup.
3. **A run with no SLO is refused, not treated as a benchmark.** Sapper asserts;
   measuring without a declared expectation is a different tool. (ADR-0001.)
4. **Secrets never reach `result.json`.** It is committed in examples and CI;
   a credential written there relocates the leak rather than reporting anything.
   Redact anything credential-shaped in captured evidence.
