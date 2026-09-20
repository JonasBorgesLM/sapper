# Sapper — Threat Model

**Scope:** the availability of the *target* and its neighbours, and the
authorization boundary around who Sapper is allowed to hit. Sapper's own secrets
matter too, but the primary asset here is unusual: it is *someone else's uptime*.

**Companion to:** [`REQUIREMENTS.md`](REQUIREMENTS.md) — every mitigation names the
requirement that discharges it, and every `SR-` appears at least once.

> A threat model that lists only the threats it defeats is marketing. §7 is what
> makes this one honest.

---

## 1. What is being protected

| Asset | Why it matters | Exposure |
| --- | --- | --- |
| Availability of the target service | Sapper's whole method is sustained load; misused, it *is* an outage | Every `run` |
| Availability of the target's dependencies | Fault injection (roadmap) interposes on real downstreams | Case 4 onward |
| The authorization boundary | The line between resilience engineering and a DoS tool | Every `run` |
| `${ENV}` secrets (target credentials) | Reused across the portfolio; `result.json` is committed | `run`, `report` |

**The asymmetry that drives the design:** load and faults are *easy to start and
hard to take back*. A scan (Warden) that hits the wrong host sends a few benign
requests; a Sapper run that hits the wrong host, or the right host too hard, can
take it down and keep it down until stopped. So the guard is on the *near* side of
every request, the caps are *required*, and the abort is *automatic* — the design
assumes the operator will eventually point it somewhere they did not mean to.

## 2. Actors and capabilities

| Actor | Assumed capability | Assumed *not* to have |
| --- | --- | --- |
| Operator (honest, fallible) | Runs Sapper, edits config, sets env | Intent to cause an outage; immunity from typos |
| CI pipeline | Runs `assert`/`run` non-interactively | Ability to answer an interactive prompt (SR-02) |
| Target service (victim) | Receives all load; may shed or 429 | Any protection from Sapper other than what BlastGuard provides |
| Target dependency (victim) | Receives injected faults (roadmap) | — |
| Bystander host (victim) | A host Sapper is *misconfigured* toward | Any reason to expect Sapper's traffic |

The victims are the point. A bystander host that receives load through a
misconfiguration is Sapper's threat even though no attacker exists — the "attacker"
here is usually the operator's own mistake.

## 3. Trust boundaries

```
   operator / CI ──[config: tier, caps, target]──▶  Sapper process
                                                        │
                                                 ┌──────┴───────┐
                                                 │  BlastGuard   │  ← every request crosses here
                                                 └──────┬───────┘
                                                        │  (only path out)
                                                        ▼
                                                   target service ──▶ its dependencies
```

- **Untrusted on the far side of config:** the target URL and tier as *typed* —
  they may be wrong. BlastGuard treats the declared tier and caps as the only
  authority to emit load.
- **Everything on the near side of BlastGuard may assume:** no request reaches the
  network without a validated tier, both caps in force, the kill switch armed, and
  the auto-abort watching. There is no second exit.

## 4. Threats

### T-01 — Load emitted around the guard
**Actor:** developer error (a new code path calls the HTTP client directly).
**Impact:** uncapped, unauthorized load — a DoS the tool is supposed to prevent.
**Mitigation:** SR-06 — BlastGuard is the *only* path to the network, wired as
mandatory middleware in the single HTTP adapter (architecture). Enforced by a test
that fails if a request can be emitted without a guard decision.
**Residual:** a genuinely separate HTTP client added in future would bypass it;
guarded by the architectural rule that there is one client, and by review.

### T-02 — Runaway load (no ceiling)
**Actor:** operator with a mistyped or absent cap.
**Impact:** load grows without bound; the target falls over.
**Mitigation:** SR-03 — max concurrency and max duration are required, positive,
validated before the first request; SR-05 — auto-abort on a latency/error ceiling.
**Residual:** the window of load *before* the ceiling trips is real; auto-abort
bounds duration, not the first spike. Stated in SR-05.

### T-03 — Wrong target (typo / stale config)
**Actor:** operator pointing at the wrong host.
**Impact:** load against a bystander service.
**Mitigation:** SR-01 — the target must carry an explicit `lab`/`staging`/
`authorized` tier; an untiered or unknown host is refused. (Mirrors Warden's
`allowed_hosts` boundary.)
**Residual:** if the wrong host is *also* tiered as authorized in a shared config,
tiering does not catch it. Reduced by requiring the tier next to the URL, not
globally. Stated here.

### T-04 — Accidental production run
**Actor:** operator or CI reusing a config against prod.
**Impact:** outage of a live service.
**Mitigation:** SR-02 — production requires a separate noisy flag *and*
interactive confirmation; CI (non-interactive) therefore cannot reach production.
**Residual:** a human who supplies the flag and confirms *can* run against prod —
that is authorized use, not a defeat. A CI job wired to a PTY could theoretically
answer the prompt; treated as out of model (T-06).

### T-05 — Fault injection hits the wrong dependency *(roadmap, Case 4)*
**Actor:** operator misconfiguring the injector.
**Impact:** a real downstream is degraded, not a test double.
**Mitigation:** deferred with the injector (ADR-0004); when built, the injector
inherits BlastGuard's tiering and caps — no fault without an authorized tier.
**Residual:** documented as open until the injector exists; **this is why Case 4
is deferred rather than shipped in v1.**

### T-06 — Deliberate misuse as a DoS weapon
**Actor:** a malicious operator.
**Impact:** intentional outage.
**Mitigation:** **none, by design.** Sapper cannot distinguish an authorized
stress test from an attack when the operator supplies real authorization; the
guard defends against *accident*, not *intent*.
**Residual:** full. Sapper is for authorized targets; misuse is the operator's
crime, the same as with curl in a loop. Stated so no one infers the guard is an
access control.

### T-07 — Secret leak via result/report
**Actor:** committing `result.json` or sharing a report.
**Impact:** target credentials disclosed.
**Mitigation:** SR-07 — `${ENV}` secrets never serialized; credential-shaped
captured values redacted.
**Residual:** a secret embedded in a *response body* the target returns is the
target's leak; Sapper redacts what it recognizes, not everything.

## 5. Coverage matrix

| Threat | Requirements |
| --- | --- |
| T-01 | SR-06 |
| T-02 | SR-03, SR-05 |
| T-03 | SR-01 |
| T-04 | SR-02 |
| T-05 | (deferred — ADR-0004) |
| T-06 | (none — out of model) |
| T-07 | SR-07 |
| — hardening — | SR-04 (kill switch: bounds every threat's duration, maps to no single one) |

Bidirectional: SR-04 appears as hardening because it shortens the exposure of
*every* threat rather than discharging one — the operator's last resort when a
run is going wrong for a reason no automatic ceiling caught.

## 6. Verification

- Each `SR-` becomes a test **validated by a negative control**: remove the guard,
  watch the test go red, restore it. A BlastGuard test never seen red is hoped.
- SR-01/SR-03: config with a missing tier / missing cap must fail load, asserted
  by the *expected error*, not the absence of a request.
- SR-04: a kill signal mid-run must stop emission within one shutdown window,
  asserted against a fake clock.
- SR-06: a "load path count" test asserts every emitted request passed a guard
  decision — assert the positive (all went through), not "none bypassed."

## 7. Explicitly not addressed

- **Deliberate misuse (T-06).** Sapper is not an access-control system; an
  authorized operator can abuse it. Whose problem: the operator's, and their
  organization's authorization policy.
- **Protecting the target *from itself*.** If the target has no rate limiter and no
  breaker, Sapper's honest finding is "it fell over." That is data, not a Sapper
  defect. Whose problem: the target's owner (often the same person — that is the
  point).
- **Network-level authorization** (firewalls, allowlists at the edge). BlastGuard
  is in-process; it does not replace network controls. Whose problem: the lab's
  network setup.
- **Third-party rate-limit / ToS violations.** Running against a host you do not
  own may breach its terms even at low volume. Whose problem: the operator.
