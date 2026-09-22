# ADR-0008: Parse OpenAPI minimally with the YAML library, not kin-openapi

## Status
Accepted

## Context
IR-01/FR-08 want a scenario to name endpoints in the target's own contract by
importing them from an OpenAPI spec. Warden does this with `kin-openapi`, a full
OpenAPI parser/validator. ADR-0006 said to reuse Warden's *approach*, not
necessarily its package, and NFR-02 makes Sapper standard-library-first, with any
dependency justified.

What Sapper actually needs from a spec is the set of `(path, method)` pairs to
aim load at — not schema validation, `$ref` resolution, or example generation. A
spec is YAML (JSON is a subset YAML parses), and Sapper already depends on
`gopkg.in/yaml.v3` for its config.

## Decision
Parse the spec's `paths` section with the YAML library into a list of
`(path, method)` endpoints. No new dependency. A scenario may then name a
`path` + `method`, and the run validates it exists in the imported spec.

## The alternative that was rejected
**Depend on `kin-openapi`** (as Warden does). It is correct and complete —
`$ref` resolution, schema validation, example values — but every one of those is
something the load path does not use, and it is a large dependency to pull into a
tool that has stayed standard-library-first. The cost is not repaid by the value
here.

## Consequences
- No `$ref` following, no schema validation, no generated request bodies. The
  import yields paths and methods only. A spec that expresses its paths through
  `$ref` indirection, or an endpoint that needs a specific body, is not fully
  handled — the parser reads the literal `paths` map and says so.
- Sapper stays dependency-light (config + this both ride `yaml.v3`).
- If a later need appears (request-body generation from schemas, `$ref` specs),
  that is the trigger to reconsider `kin-openapi` — recorded below.

## Reopening criterion
A scenario needs request bodies generated from the spec's schemas, or must
import a spec that relies on `$ref` indirection for its paths. Adopt
`kin-openapi` then, for those specs.
