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
| [`0004`](0004-defer-fault-injector.md) | Defer the fault injector; reserve the port in v1 | Accepted | `0007` |
| [`0005`](0005-warden-sapper-naming.md) | Warden inspects, Sapper mines; scanner renamed to Warden | Accepted | — |
| [`0006`](0006-reuse-from-warden.md) | Reuse Warden's config style, OpenAPI import, and JSON pipeline | Accepted | — |
| [`0007`](0007-fault-injection-approach.md) | Build a minimal in-process fault proxy, not integrate toxiproxy | Accepted | — |
| [`0008`](0008-minimal-openapi-parser.md) | Parse OpenAPI minimally with the YAML library, not kin-openapi | Accepted | — |

## Reopening criteria on the record

| ADR | Reopen when |
| --- | --- |
| `0007` | A scenario needs a fault type the minimal proxy cannot express (bandwidth, slicer, partial writes) — integrate toxiproxy then |
| `0008` | A scenario needs request bodies generated from schemas, or a spec using `$ref` indirection — adopt kin-openapi then |
| `0003` | A metrics histogram dependency is chosen — record it in its own ADR |

## Open questions

- ~~Own vs. external (toxiproxy) fault-injection proxy.~~ Decided by
  [`0007`](0007-fault-injection-approach.md): build a minimal in-process proxy.
- **Non-HTTP protocols (gRPC, …).** No ADR yet; add one if a requirement appears.
