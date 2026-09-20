# ADR-0005: Warden inspects, Sapper mines; the scanner is renamed to Warden

## Status
Accepted

## Context
The scanner and Sapper are a pair in a stone-fortress naming world (`bastion`,
`moat`, `cairn`, `crier`). The scanner *inspects* the walls; Sapper *mines* them
to find where they collapse. "Security-scanner" is a description, not a name, and
the pair reads best as **Warden** (the warden who patrols and inspects) and
**Sapper** (the sapper who undermines). The scanner's repository is public as
`security-scanner`, so a rename there has real cost: module path, package,
README, and any external links.

## Decision
- **Within Sapper's documentation, the sibling is called Warden**, with the repo
  named parenthetically (`Warden (repo: security-scanner)`) until the rename lands.
- The scanner-side rename to **Warden** is the **accepted direction** (owner's
  decision). Executing it — changing `github.com/JonasBorgesLM/security-scanner`'s
  module path, package name, and README — is **separate work in that repository**
  and is not performed as part of Sapper's setup.
- Cross-project requirement citations from Sapper use the qualified prefix
  `warden/` (e.g. `warden/IR-06`), which is where the module will land.

## The alternative that was rejected
**Keep calling it "the scanner" and record no naming decision.** Rejected: the
pairing is the clearest way to explain what Sapper *is* (its complement), and
leaving the sibling unnamed makes the README's central analogy clumsy. The
zero-friction "product name in README only, decision left open" option was also
considered and rejected because the owner chose to treat the rename as decided.

## Consequences
- Sapper's docs reference a name the scanner's repo does not yet carry; the
  parenthetical repo name bridges the gap until the rename lands.
- If the scanner rename is later abandoned, this ADR is **amended** (not edited)
  and Sapper's `warden/` citations and prose are revised.

## Reopening criterion
The scanner rename is executed (drop the parenthetical) or explicitly abandoned
(amend this ADR).
