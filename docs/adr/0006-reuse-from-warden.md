# ADR-0006: Reuse Warden's config style, OpenAPI import, and JSON pipeline

## Status
Accepted

## Context
Sapper and Warden solve adjacent problems and will often run against the same lab
target. Warden has already settled several things that Sapper needs: a config
style (YAML + `${ENV}`, `schema_version`, a hard scope boundary), an OpenAPI
import that turns a spec into concrete targets with methods and sample values, and
an independent-subcommands-chained-through-JSON pipeline shape. Reinventing these
differently would make the pair inconsistent for no gain and double the
maintenance of two config dialects.

## Decision
Sapper **adopts Warden's conventions** for: config format and `${ENV}` secret
handling; target resolution via OpenAPI import; and the subcommand/JSON-on-disk
pipeline. It reuses the *approach and property*, not necessarily a shared code
package — the dependency is on "a scenario names endpoints in the target's own
contract" (IR-01), not on importing `security-scanner`'s parser.

## The alternative that was rejected
**Extract a shared library** the two tools both import. Rejected as premature: two
projects is not enough evidence for the right shared abstraction (duplication is
cheaper than the wrong abstraction), and a shared package couples their release
cycles. If a third consumer appears or the two drift painfully, revisit — copying
the approach now keeps them independent.

## Consequences
- Some parsing/config code is similar across the two repos (duplication). Accepted
  deliberately; it lets each evolve without breaking the other.
- An operator who knows Warden's config already knows Sapper's — the extra fields
  (tier, caps) are the only new surface.

## Reopening criterion
A third tool needs the same import/config, or the two implementations drift enough
that keeping them consistent by hand costs more than a shared module would.
