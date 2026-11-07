# ADR-0010: Requests may carry a body with per-request variables, and statuses are asserted by absence and fraction

## Status
Accepted

## Context
Sapper could only replay one bodiless request, with static headers. That is
enough for a GET behind a rate limiter, and not for the case that matters most
for a write endpoint: a service whose correctness is idempotency (FR-10 could
send an `Idempotency-Key`, but the *same* one every time, so after the first
request the target answers a replay and the run measures its cache, not its
write path). Two assertions were also missing. `error_rate_under` counts
transport failures only, and a 5xx is "a status, not an error" (collector), so
"the service never answered 500 under load" could not be stated. And
`status_seen` asserts presence; shedding load with 503 is legitimate, but
shedding most of it is not, and nothing bounded that.

The first user is a ledger service (the `wagering-processor`) that must be
stressed with POSTs carrying a unique id and key per request.

## Decision
1. `request.body`: a map or list is encoded as JSON at load (Content-Type
   defaults to `application/json` unless a header sets it); a string is sent
   as written. It is not env-expanded: it is test data, not a credential.
2. `{{uuid}}` and `{{seq}}` in path, header values and body, filled per request.
   `uuid` is a v4 UUID, one per request (every occurrence in that request is the
   same value); `seq` is a run-wide counter shared by all workers and phases.
   An unknown `{{name}}` fails the load.
3. `status_absent: <code>` passes when no response answered the code;
   `status_fraction_under: {status, fraction}` passes when that status is
   strictly below the fraction of responses. Transport failures are not
   responses, so they are outside both (they remain `error_rate_under`'s).
   Both fail when the run recorded no response.

## The alternative that was rejected
A scripting hook (a Go plugin or an expression language) for request
generation. It would cover any generator, at the price of an interpreter inside
a tool whose safety story is that every request goes through one guarded
client with a small, reviewable surface (ADR-0002). Two variables cover
uniqueness, which is the one thing a replayed request cannot fake; a third
need (a pool of ids, a random amount) is the signal to reconsider, not a reason
to build for it now.

A second alternative, a distinct syntax for nothing, was also rejected:
reusing `${VAR}` for per-request values. `${VAR}` is resolved once at load from
the environment and must fail when unset; mixing a per-request meaning into it
would make an unset secret indistinguishable from a request variable.

## Consequences
- A body is in memory text, rendered per request: one allocation per request on
  the hot path, negligible against the network call.
- Two same-scenario runs send different bodies; runs are statistically, not
  byte-for-byte, comparable, as ADR-0003 already says.
- A path using `{{uuid}}` does not match a templated `/x/{id}` in a configured
  OpenAPI spec, so the spec check (IR-01) will reject it; use a spec-less
  config for such scenarios.
- Absence is stated over responses, so a run that cannot reach the target fails
  rather than "passes with no 500".

## Reopening criterion
A scenario needs a value the two variables cannot give (a pool of existing ids,
a random choice, an amount distribution).
