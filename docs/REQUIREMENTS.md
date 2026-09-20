# Sapper — Requirements

**Version:** 0.1
**Status:** baseline for implementation — no code written yet

This document is binding. Every `FR-`, `SR-`, `NFR-` and `IR-` identifier below
is cited from commit messages, ADRs, godoc and tests.

> **A requirement without a test that fails when the protection is removed counts
> as unimplemented.**

Cross-project citations are written qualified — `warden/IR-06`, `bastion/…` — so
they are not silently checked against this project's numbering.

---

## 1. Purpose

Sapper applies sustained adversarial load, and (later) controlled fault
injection, to an authorized API target, and **asserts the observed behaviour
against SLOs declared in the scenario** — producing a green/red verdict that is
gateable in CI. It answers correctness-under-stress questions that a single probe
cannot create, complementing the Warden scanner.

### 1.1 Non-goals

- **Throughput benchmarking without an assertion.** Owned by k6/Locust. A Sapper
  run with no declared SLO is a misconfiguration. (ADR-0001.)
- **Fuzzing / malformed-input exploration.** A different question (input, not
  load/time); a separate tool owns it.
- **Protocols other than HTTP(S)** in v1. Owned by a future requirement if one
  arises.
- **Being a load *service*.** Sapper is a CLI run by a human or CI, not a
  long-lived daemon.

---

## 2. Actors

| Actor | Role | Assumed capability | Assumed *not* to have |
| --- | --- | --- | --- |
| Operator | Runs Sapper from a shell or CI against a target they own | Can set config, env vars, and read exit codes | Authority to hit arbitrary third-party hosts; intent to cause an outage |
| CI pipeline | Runs `assert` as a gate | Reads exit code and `result.json` | Interactive confirmation ability (so it can never clear the production gate) |
| Target service | The API under test | May be fronted by bastion/a rate limiter | Any expectation that load is unbounded or safe |
| Target's dependency | A datastore/downstream the target calls | — | (Roadmap) Immunity from the fault injector |

The last column is the one people skip and the one that matters: CI **cannot**
answer an interactive prompt, which is precisely why the production gate is
interactive (SR-02).

---

## 3. Functional requirements

- **FR-01** `sapper run` executes a scenario against a target and writes a single
  JSON result file (metrics, per-status counts, timeline, config echo, verdict
  inputs).
- **FR-02** A scenario declares a **load profile** — one of `sustained`,
  `ramp-up`, `spike`, `soak` — with its parameters (rate/concurrency, duration,
  ramp shape).
- **FR-03** A scenario declares one or more **SLOs** (e.g. `p99 < Z ms`,
  `status==429 above X rps`, `nothing passes above Y rps`, `zero 5xx from
  unhandled panic`).
- **FR-04** `sapper assert` reads a result file and produces a green/red verdict
  against its SLOs, exiting non-zero on any red SLO so CI can gate on it.
- **FR-05** `sapper report` reads a result file and renders an HTML report:
  latency percentiles, variance, per-status breakdown, and a timeline. It never
  touches the network.
- **FR-06** The metrics collector records a latency **histogram** (percentiles),
  per-status counts, and error rate, cheaply enough not to distort the
  measurement it takes. (ADR-0003.)
- **FR-07** A run performs a **warm-up** whose samples are excluded from the
  reported statistics, and reports across **N runs** with percentiles + variance
  — never a single "best" run. (ADR-0003.)
- **FR-08** A scenario is imported/resolved to concrete HTTP targets, reusing
  Warden's OpenAPI-import and target-resolution approach. (IR-01, ADR-0006.)
- **FR-09** *(Roadmap, Case 4)* A **fault-injector** interface lets a scenario
  inject latency/error/drop into a target's dependency without touching the
  target's code. v1 defines the port only; no implementation. (ADR-0004.)

One sentence each. Anything needing a paragraph is two requirements or an ADR.

---

## 4. Security requirements

Security here is not about protecting Sapper's data — it is about **preventing
Sapper from causing an unauthorized outage**. This section is the point of the
tool's discipline. Each `SR-` is discharged by BlastGuard (see architecture) and
mapped in the threat model.

- **SR-01** **Environment tiering.** Sapper refuses to run against a target not
  explicitly marked `lab` | `staging` | `authorized`. The tier is a **required**
  config field with no default.
- **SR-02** **Production gate.** Targeting production requires a separate,
  explicit flag *and* interactive confirmation. A non-interactive context (CI)
  therefore cannot run against production. *Partial:* this defends against
  accident, not against a determined operator who supplies the flag and answers
  the prompt — that is out of the model (T-06).
- **SR-03** **Mandatory blast-radius caps.** Maximum concurrency and maximum
  duration are **required** config fields, validated as positive before the first
  request; a missing/zero/negative value aborts startup with a clear error.
- **SR-04** **Kill switch.** SIGINT/SIGTERM aborts all load immediately via a
  graceful generator shutdown; no in-flight ramp can outlive the signal.
- **SR-05** **Catastrophic auto-abort.** When error rate or latency crosses a
  configured ceiling, the run stops on its own and records why. *Partial:* it
  bounds damage *after* onset within one sampling window; it does not prevent the
  window of load that revealed the threshold.
- **SR-06** **No load bypasses BlastGuard.** All request emission goes through the
  guard middleware; there is no code path to the target around it. (Enforced
  structurally — architecture — and by test.)
- **SR-07** **Secret hygiene.** `${ENV}`-sourced secrets never appear in
  `result.json` or the HTML report; credential-shaped captured values are
  redacted. `result.json` is committed in examples/CI.

Where a mitigation is partial it says so **here**, not only in the threat model.

---

## 5. Non-functional requirements

- **NFR-01** Go language floor `1.25.0` (matches `security-scanner`, `bastion`);
  build `toolchain` pinned in `go.mod`. This floor is a **choice** for ecosystem
  consistency, not an imposition by a dependency.
- **NFR-02** Standard-library-first. Any external dependency is justified in an
  ADR. The latency histogram in particular must be low-overhead (FR-06).
- **NFR-03** Lightweight hexagonal structure (ports/adapters) so the generator,
  collector, and BlastGuard are testable against fakes with no real network.
- **NFR-04** Deterministic *structure* despite non-deterministic *timing*: given
  the same config, the same requests are constructed and the same SLOs evaluated;
  only measured latencies vary. (ADR-0003.)
- **NFR-05** CI runs build, vet, lint (pinned toolchain), and tests; `sapper
  assert` is itself demonstrated as a gate in CI against a fixture result.
- **NFR-06** Subcommands are independent stages communicating only through JSON
  on disk; no hidden shared state.

For a version floor, whether it is a choice or an imposition is stated (NFR-01).

---

## 6. Integration requirements

Name the *property* depended on, not today's library.

- **IR-01** Sapper resolves targets the same way Warden does — an OpenAPI import
  yields concrete endpoints with methods and sample values. (Property: "a scenario
  names endpoints in the target's own contract," not "we depend on
  `security-scanner`'s parser package.")
- **IR-02** *(Roadmap, Case 4)* When pointed at a **bastion**-wrapped service, an
  injected dependency fault is expected to drive `bastion/`'s breaker open; Sapper
  asserts the breaker opens and the service recovers. The property depended on is
  "the wrapper opens after N consecutive failures and half-opens after a cooldown"
  — `bastion/`'s documented contract — not a specific bastion version.
- **IR-03** A rate-limiter target (gateway/middleware) is expected to answer `429`
  above its configured rate and to let **nothing** through above the hard ceiling;
  Sapper asserts both. Property: "the limiter's key derives from the untrusted
  peer and the ceiling is enforced server-side."

---

## 7. Out of scope

Deferred deliberately, so an undecided question does not look decided.

- **Fault injection implementation** — port defined in v1, no adapter. Reopens per
  [ADR-0004](adr/0004-defer-fault-injector.md).
- **Own vs. external fault-injection proxy** — the build/buy choice is itself
  deferred with the injector. [ADR-0004](adr/0004-defer-fault-injector.md).
- **Non-HTTP protocols** — no ADR yet; add one when a requirement appears.
- **Distributed/multi-node load generation** — single-process only until a
  measured need exists.

---

## 8. Acceptance (v1 = Case 1)

v1 is the `sustained` scenario with a p99 assertion, proven end to end. Each item
is checkable, not aspirational.

1. An operator can `run` a `sustained` scenario against a lab target and get a
   `result.json`; `assert` turns its `p99 < Z` SLO green/red with a matching exit
   code; `report` renders the percentiles, variance, and timeline. (FR-01, FR-03,
   FR-04, FR-05, FR-06, FR-07.)
2. Every `SR-` is covered by a test **seen to fail** with the protection removed —
   in particular SR-01, SR-03, SR-04, SR-06.
3. `sapper run` against an untiered target, or with a missing cap, or with no SLO,
   is refused with a clear error (SR-01, SR-03, ADR-0001).
4. The ADR index lists no open question the code has already answered by accident.

**Cases 2–3** (`ramp-up`→429, `spike`) reuse this spine with no new subsystem and
are the immediate follow-on. **Cases 4–5** (fault injection, `soak`) are roadmap.
