# Sapper — Architecture

**Status:** design, pre-code. Signatures and boundaries below are the intended
shape; there are no bodies yet. Read this and [`THREAT-MODEL.md`](THREAT-MODEL.md)
before writing implementation — the safety properties are structural.

Lightweight hexagonal (ports/adapters), the same shape as Warden
(`security-scanner`), so the generator, collector, and BlastGuard are testable
against fakes with no real network.

---

## 1. Shape at a glance

```
cmd/sapper                    CLI: run | assert | report  (thin; parse → call core → write JSON)
  │
  ├─ internal/core/scenario   load profiles + SLOs, parsed from YAML (+ ${ENV})
  ├─ internal/core/model      shared types: Result, Sample, Verdict, Target, Tier, Caps
  ├─ internal/core/generator  the load engine — drives a profile, emits requests
  │     └─ BlastGuard          MANDATORY middleware; the only path to the network
  ├─ internal/core/metrics    latency histogram, per-status counts, error rate
  ├─ internal/core/assert      evaluate SLOs against a Result → Verdict
  ├─ internal/core/report      Result → HTML (no network)
  │
  ├─ internal/ports           interfaces only: Clock, Requester, (reserved) FaultInjector
  └─ internal/adapters
        ├─ httpclient          the one real client; BlastGuard is wired in here
        ├─ openapi             OpenAPI spec → []Target   (reuses Warden's approach)
        └─ (reserved) inject   fault-injector adapter — NOT built in v1
```

Dependencies point inward: adapters depend on ports and model; core depends on
ports and model; nothing in core imports an adapter. `cmd/sapper` is the only
place that wires a real adapter into core.

## 2. The load generator and BlastGuard

The generator is the core of the tool, and **BlastGuard is the core of the
generator** — the structural mirror of Warden's `ScopeGuard`. The design rule,
from ADR-0002:

> The guard is wired as middleware in the single HTTP adapter. There is exactly
> one `Requester` reaching the network, and it is the guarded one. Core code is
> handed that `Requester`; it has no way to construct an unguarded one.

Intended port and wiring (illustrative, not final):

```go
// internal/ports
type Requester interface {
    Do(ctx context.Context, req *Request) (*Response, error)
}

// internal/adapters/httpclient
// NewGuarded is the ONLY constructor cmd/sapper calls. There is no exported
// path to an unguarded client — that is what makes "no load bypasses the guard"
// (SR-06) a compile-time property, not a convention.
func NewGuarded(cfg GuardConfig, inner *http.Client) (ports.Requester, error)
```

BlastGuard's decision, evaluated before every request and continuously during the
run:

| Check | Requirement | When |
| --- | --- | --- |
| Tier is `lab`/`staging`/`authorized` (or prod-gate cleared) | SR-01, SR-02 | at construction; fail fast |
| Both caps present and positive | SR-03 | at construction; fail fast |
| Concurrency within cap | SR-03 | per request |
| Elapsed within max duration | SR-03 | per request |
| Kill signal not received | SR-04 | per request; drains on signal |
| Error/latency below abort ceiling | SR-05 | per sampling window |

Fail-fast checks abort startup with a clear error; per-request checks stop
emission and mark the run aborted-by-guard, recording *why* in the `Result`.

## 3. Load profiles (the scenario engine)

A scenario is YAML: a profile + its parameters + SLOs. Profiles:

- **`sustained`** — constant rate/concurrency for a duration. *(v1)*
- **`ramp-up`** — increase toward a target rate; used to find the rate-limiter
  knee. *(Case 2)*
- **`spike`** — baseline, sudden peak, back to baseline; asserts recovery.
  *(Case 3)*
- **`soak`** — moderate load, long duration; leak hunting. *(Case 5)*

Each profile is a strategy that tells the generator *how many* requests to have
in flight *when*; the generator, the guard, the collector, and the SLO evaluator
are shared across all profiles. Adding a profile adds a strategy, not a subsystem
— which is why Cases 2–3 are "the same spine, no new subsystem."

## 4. Metrics and statistical honesty

Determinism byte-for-byte is impossible (p99 varies), and is replaced by
statistical honesty (ADR-0003): a **warm-up** window whose samples are discarded,
**N runs** aggregated, and **percentiles + variance** reported — never a single
best run. The collector uses an HDR-style latency histogram; it must be cheap
enough not to distort what it measures (FR-06, NFR-02) — the one place an external
dependency is most likely justified, and it needs its own ADR when chosen.

## 5. The assertion model

`assert` is a pure function of a `Result`: it evaluates each SLO and produces a
`Verdict` (per-SLO green/red + reasons), and the CLI maps any red to a non-zero
exit. Because it is pure and reads only the JSON, CI gates on it without a target
present, and a report can be regenerated from an old result. SLOs on a percentile
must be evaluated against the reported variance, or the gate flaps.

## 6. The reserved fault-injector seam

Case 4 (fault injection, the bastion tie-in) is **deferred** (ADR-0004). The
architecture reserves the boundary and builds nothing behind it:

```go
// internal/ports — defined in v1, no adapter implements it yet.
type FaultInjector interface {
    Inject(ctx context.Context, fault Fault) (revert func(), error)
}
```

When built, the injector adapter is wired the same way as the HTTP client — behind
BlastGuard's tiering and caps, so a fault cannot be injected against an
unauthorized tier (T-05). Reserving the port now keeps the generator and result
model from having to change shape when the injector arrives; building the port's
*body* now would be speculative (YAGNI) and would add threat surface v1 does not
need.

## 7. The pipeline (subcommands)

Independent stages, chained through JSON on disk — Warden's philosophy, no hidden
shared state:

```
sapper run    --scenario s.yaml --config c.yaml --out result.json
sapper assert --in result.json                     # exit != 0 on red SLO
sapper report --in result.json --out report.html   # never touches the network
```

`result.json` carries: the config echo (tier, caps, target — secrets redacted),
the metrics, per-status counts, the timeline, the SLOs as declared, and whether
the run completed or was aborted (and by which guard check).

## 8. Configuration contract

YAML + `${ENV}` for secrets, with a `schema_version`, mirroring Warden. The
loader **fails loudly** when a required safety field is absent — no default tier,
no default cap.

```yaml
schema_version: 1

target:
  base_url: http://localhost:8080
  tier: lab            # REQUIRED: lab | staging | authorized | production
                       # 'production' additionally requires --i-know + interactive confirm

blast_radius:          # REQUIRED block — no silent defaults
  max_concurrency: 50  # REQUIRED, positive
  max_duration: 60s    # REQUIRED, positive
  auto_abort:
    error_rate_over: 0.5     # stop if >50% errors in a window
    p99_over: 2s             # stop if p99 exceeds this in a window

auth:                  # optional; same shape as Warden (secrets via ${ENV})
  login_endpoint: /login
  credentials:
    username: admin
    password: ${LAB_PASSWORD}
  token_path: data.access_token
```

The scenario file (load profile + SLOs) is separate from the config (target +
safety), so the same safety config can guard many scenarios and a scenario can be
reused across targets.
