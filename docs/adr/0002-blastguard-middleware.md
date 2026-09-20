# ADR-0002: BlastGuard is mandatory middleware in the one HTTP client

## Status
Accepted

## Context
Sapper's method is sustained load and (later) fault injection. The distance
between "resilience test harness" and "DoS weapon" is entirely the authorization
and safety discipline. If that discipline is a caveat in the docs or a check the
caller is expected to call, it will be bypassed — by a new code path, by a
refactor, by a hurried change — and the first time it is bypassed is an outage.
Warden faced the same problem with scope and solved it with `ScopeGuard` wired
into its single HTTP adapter.

## Decision
BlastGuard is **middleware in the one and only HTTP client**, and there is **no
exported constructor for an unguarded client**. Core code is handed a
`ports.Requester`; the only implementation reaching the network is the guarded
one, constructed in `cmd/sapper`. "No load bypasses the guard" (SR-06) is thus a
structural property — closer to compile-time than to convention. The guard checks
tier, both caps, the kill switch, and the auto-abort ceiling (see architecture §2).

## The alternative that was rejected
**A `guard.Check()` function the generator calls before each request.** Rejected:
it is a convention, and a convention nobody structurally enforces is a preference
(foundation rule 3). The next request-emitting code path forgets the call, the
tests still pass, and the guarantee is gone silently. Middleware on the sole
client cannot be forgotten because there is nothing else to call.

## Consequences
- One client, one seam. A future need for a genuinely separate client (a second
  protocol) must re-establish the guarantee explicitly and gets an ADR.
- Testing the guard means testing the client wrapper against a fake inner client
  and a fake clock — cheap, no network.
- Slight ceremony: even a one-off internal request goes through the guarded
  client. Accepted; that is the invariant working.

## Reopening criterion
A second network client (e.g. non-HTTP transport) forces the question of how the
guarantee extends; reopen then.
