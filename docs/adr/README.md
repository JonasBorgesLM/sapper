# Architecture Decision Records

Every structural decision is recorded here. **An ADR is never edited to reflect a
later change of mind.** It is amended in place with a section naming what
superseded which part, so the reasoning that was current at the time stays
readable.

New records start from [`TEMPLATE.md`](TEMPLATE.md).

## Index

| ADR | Title | Status | Amended by |
| --- | --- | --- | --- |
| [`0001`](0001-assertion-not-benchmark.md) | Sapper asserts against SLOs; it is not a benchmark | Accepted | — |
| [`0002`](0002-blastguard-middleware.md) | BlastGuard is mandatory middleware in the one HTTP client | Accepted | — |
| [`0003`](0003-statistical-honesty.md) | Statistical honesty replaces byte-for-byte determinism | Accepted | — |
| [`0004`](0004-defer-fault-injector.md) | Defer the fault injector; reserve the port in v1 | Accepted | — |
| [`0005`](0005-warden-sapper-naming.md) | Warden inspects, Sapper mines; scanner renamed to Warden | Accepted | — |
| [`0006`](0006-reuse-from-warden.md) | Reuse Warden's config style, OpenAPI import, and JSON pipeline | Accepted | — |

## Reopening criteria on the record

| ADR | Reopen when |
| --- | --- |
| `0004` | Case 4 (breaker validation) is scheduled — decide build-own vs. external proxy then |
| `0003` | A metrics histogram dependency is chosen — record it in its own ADR |

## Open questions

- **Own vs. external (toxiproxy) fault-injection proxy.** Deferred with the
  injector itself; see `0004`. Recorded here so it does not look decided.
- **Non-HTTP protocols (gRPC, …).** No ADR yet; add one if a requirement appears.
