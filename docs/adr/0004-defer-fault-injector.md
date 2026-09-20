# ADR-0004: Defer the fault injector; reserve the port in v1

## Status
Accepted

## Context
Case 4 — inject a dependency fault, assert a bastion-wrapped service's circuit
breaker opens and recovers — is the case that ties Sapper to the rest of the
portfolio, and the most compelling demo. It is also the largest new subsystem: a
fault-injecting proxy (toxiproxy-style) interposed between the target and its
dependency, with its own blast radius (it degrades a *real* downstream) and its
own build-vs-buy question. Building it in v1 would delay the milestone that proves
the whole spine (Case 1) and would add threat surface (T-05) before the load path
is even trusted.

## Decision
v1 **defines the `FaultInjector` port and no adapter**. The generator and result
model are designed so the injector can arrive without reshaping them. The
build-own vs. integrate-toxiproxy choice is **deferred with the implementation** —
deciding it now would be deciding an interface for code no one has tried to write.
When the injector is built, it is wired behind BlastGuard's tiering and caps, so a
fault cannot target an unauthorized tier.

## The alternative that was rejected
**Build the injector in v1** so the bastion tie-in ships early. Rejected on blast
radius and sequencing: the injector damages real downstreams, and shipping it
before the load path is proven means debugging two unproven subsystems at once.
The tie-in is the reward for a working spine, not the first milestone.

## Consequences
- The most exciting case is not in v1. Accepted — the roadmap and README are
  explicit, so it does not read as an omission.
- Reserving the port risks designing an interface that the real implementation
  fights. Mitigated by keeping the port minimal (`Inject → revert`) and treating
  it as provisional until Case 4 validates it.

## Reopening criterion
Case 4 is scheduled. At that point: (1) decide build-own vs. external toxiproxy,
in a new ADR; (2) revisit whether the reserved port shape survived contact.
