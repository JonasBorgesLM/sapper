# Sapper

**A resilience test harness for Go APIs: sustained adversarial load plus
controlled fault injection, asserted against a declared expectation — pass or
fail.** For authorized lab and staging targets only.

> **Status: v1 (Case 1) works.** The sustained scenario runs end to end —
> `run` → `assert` → `report` — with BlastGuard enforcing the safety invariants.
> Ramp-up, spike, chaos injection and soak are on the [roadmap](#roadmap) below.
> See [`docs/`](docs/) for the design.

Sapper is the offensive complement to **Warden**, the black-box API security
scanner ([repo: `security-scanner`](https://github.com/JonasBorgesLM/security-scanner)).
Where Warden asks *"does this control exist and is it configured well?"* with a
single gentle probe, Sapper asks the **same family of correctness questions under
conditions a single probe cannot create**: *"does this control hold under
sustained adversarial pressure — and when it breaks, does it fail open or
closed?"*

- Warden asks whether a rate limiter exists → Sapper asks whether it holds under
  a burst and real concurrency, and whether anything leaks past the ceiling.
- Warden asks whether a service handles an error → Sapper drops a dependency and
  asks whether the circuit breaker opens, sheds load, and recovers.

## The thesis

The opposite of *gentle* is not *aggressive* — it is *without purpose*. Sapper is
aggressive **with** purpose: volume and time are the method, never the goal. It
is not a load generator that happens to have thresholds. It is an **assertion
tool**: every scenario declares SLOs, and the result is green or red against
them, gateable in CI exactly like Warden. That is what keeps it about
*correctness under stress* rather than throughput, and it is what keeps it out of
the k6/Locust space — those tools *measure* capacity; Sapper *affirms that a
defense works*. Different question.

## Aggressive with load, disciplined with authorization

A stress tool that cannot stop itself is a bad tool. The line between resilience
engineering and a DoS weapon is the authorization discipline, so it is built into
the load path, not bolted on. Every unit of load passes through **BlastGuard** —
the mandatory middleware in the generator, the same principle as Warden's
`ScopeGuard`: it is impossible to generate load without going through it.

- **Environment tiering** — refuses to run against a target not marked
  lab/staging/authorized. Production requires a separate, noisy flag with
  interactive confirmation.
- **Kill switch** — a signal (Ctrl+C) aborts all load immediately via a graceful
  generator shutdown.
- **Catastrophic auto-abort** — if error rate or latency crosses a configured
  ceiling, Sapper stops on its own. Hammering an already-dead service is useless
  data and gratuitous damage.
- **Mandatory blast-radius caps** — maximum concurrency and maximum duration are
  both *required* in config. No silent, dangerous default.

## The flow

Independent subcommands, chained through JSON on disk — the same shape as Warden:

```
sapper run    --scenario spike.yaml --config config.yaml --out result.json
sapper assert --in result.json     # green/red against the scenario's SLOs (CI-gateable)
sapper report --in result.json     # HTML: percentiles, variance, timeline
```

## Where it fits in the portfolio

Sapper is the natural test harness for the defenses already built:

- **[`bastion`](https://github.com/JonasBorgesLM/bastion)** (circuit breaker, retry,
  timeout) — point Sapper at a bastion-wrapped service, inject a dependency fault,
  and *assert* the breaker opens and recovers. (Roadmap — Case 4.)
- **`gateway-auth` / middleware libraries** (rate limiting) — validate the limiter
  under a real burst and concurrency.

## Roadmap

Ordered smallest blast radius to largest; each case has a clear assertion.

1. **`sustained` + p99 assertion** — simplest scenario; proves collector + engine
   + assert + report end to end. **This is v1.**
2. **`ramp-up` to 429** — find the rate limiter's knee; assert 429 appears and
   nothing passes above the ceiling.
3. **`spike`** — sudden peak; assert recovery (latency returns to baseline).
4. **Fault injection + circuit breaker** — drop a dependency through the injector
   seam; assert the breaker opens and the service degrades gracefully. *Ties to
   `bastion`.* (Deferred; the injector port is reserved in the architecture.)
5. **`soak`** — moderate load over long duration; hunt memory/connection leaks.

## Explicitly out of scope

- **Fuzzing** — that is exploration of malformed *input*, not *load/time*. A
  different question, a separate tool.
- **Throughput benchmarking as an end in itself** — that is k6. Sapper always
  asserts against a declared expectation.
- **Running against production without the noisy gate.** Never. BlastGuard is
  non-negotiable.

## Documentation

| | |
| --- | --- |
| [`docs/REQUIREMENTS.md`](docs/REQUIREMENTS.md) | What it must do, with cited ids. |
| [`docs/THREAT-MODEL.md`](docs/THREAT-MODEL.md) | What BlastGuard protects, and the honest residuals. |
| [`docs/architecture.md`](docs/architecture.md) | Packages, the BlastGuard middleware, the reserved injector port. |
| [`docs/adr/`](docs/adr/) | The decisions and their trade-offs. |
