# ADR-0007: Build a minimal in-process fault proxy, not integrate toxiproxy

## Status
Accepted

## Context
Case 4 (the bastion tie-in) needs to inject faults into a target's dependency —
error responses, added latency, and refused/dropped connections — so Sapper can
assert that a bastion-wrapped service's circuit breaker opens under failure and
recovers afterwards. ADR-0004 reserved the `FaultInjector` port and **deferred
the build-vs-buy choice** to the point Case 4 was scheduled. It is now scheduled,
so this is that decision.

The faults the breaker test actually exercises are a small set: return a 5xx,
add latency (up to a timeout), and refuse/close the connection. It does not need
bandwidth shaping, packet slicing, or the rest of a full network-fault toolkit.

## Decision
Build a **minimal in-process fault proxy** in Sapper: a std-library reverse
proxy (`net/http/httputil`) that sits between the target and its dependency,
with a controllable fault mode injecting the three faults above. The target is
pointed at the proxy's address for the run; Sapper drives the fault mode
directly, in the same process, behind BlastGuard's tier gate — so a fault cannot
be injected against an unauthorized tier (T-05), the same in-process guarantee
the load path already has.

## The alternative that was rejected
**Integrate toxiproxy** (Shopify's fault-injection proxy), running it as a
separate server and driving it through its HTTP API or Go client. It is mature,
battle-tested, and offers far richer "toxics" — latency, timeout, bandwidth,
slicer, reset_peer — that our proxy will not have.

Rejected on cost, not merit. It adds a **separate runtime** the operator must
start and wire alongside every run, plus a client dependency, to a tool that is
otherwise a single self-contained binary in an ecosystem that prizes zero
dependencies (`bastion` ships with none). And the faults it adds beyond our three
are ones the breaker test does not use. Its tiering story is also looser: the
faults happen in another process, so BlastGuard governs them only indirectly,
whereas the in-process proxy inherits the tier gate directly (T-05). For a study
tool testing a breaker with three fault types, the operational weight outweighs
the richer toxics.

## Consequences
- We own the proxy and its edge cases (connection handling, timeout semantics,
  clean shutdown). This is real code, and getting a TCP-level "drop" exactly
  right is harder than an HTTP-level error — the injector will start with what
  a reverse proxy can express cleanly and say where it approximates.
- No bandwidth/slicer/partial-write toxics. Accepted: no scheduled case needs
  them.
- Sapper stays a single binary with no new runtime to operate, and the fault
  injector sits behind the same in-process guard as the load.

## Reopening criterion
A scenario needs a fault type the minimal proxy cannot express well —
bandwidth throttling, byte-level slicing, or realistic partial writes. At that
point, integrate toxiproxy for those toxics rather than reinventing them.
