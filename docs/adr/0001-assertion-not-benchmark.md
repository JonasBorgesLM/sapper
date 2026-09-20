# ADR-0001: Sapper asserts against declared SLOs; it is not a benchmark

## Status
Accepted

## Context
A tool that applies heavy load looks, from the outside, exactly like k6 or
Locust. Those tools *measure* — they answer "how many requests per second can
this take?" and hand back a number. There is a strong gravity toward becoming
that: it is the obvious thing a load generator does. If Sapper becomes a
velocimeter, it competes with mature tools on their turf and loses its reason to
exist.

## Decision
Every scenario **declares SLOs**, and the primary output of a run is a green/red
**verdict** against them (`sapper assert`, exit code gated). A run with no SLO is
a **misconfiguration and is refused**, not silently treated as a benchmark. The
question Sapper answers is *"does the defense hold under this stress?"* — a
correctness question — not *"how fast is it?"*.

## The alternative that was rejected
**Ship a benchmark mode** (load + report, no assertion) alongside the assertion
mode. Rejected: it reintroduces exactly the identity Sapper is defined against,
splits the tool's purpose, and invites comparison with k6 on throughput numbers
Sapper is not trying to win. The measurement is still produced — the report has
percentiles and variance — but it is *in service of* the assertion, never the end.

## Consequences
- Sapper is useless for pure capacity planning; that is intended, and the README
  says so. Reach for k6 there.
- Authoring a scenario costs more than a k6 script: you must state the expectation.
  That cost is the point — an un-stated expectation is a gate that passes on
  anything.
- `assert` being a pure function of `result.json` makes CI gating and report
  regeneration fall out for free.

## Reopening criterion
If a concrete, recurring need for un-asserted measurement appears that k6 cannot
serve, reopen. "It might be handy" is not that trigger.
