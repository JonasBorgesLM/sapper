// Command sapper is the resilience test-harness CLI: run | assert | report.
//
// Pre-implementation scaffold. The subcommands are not wired yet; this main
// exists so the module builds and the layout is in place (issue #1). Behaviour
// arrives with the Phase 1 issues.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "sapper: not implemented yet — see docs/ and the project backlog")
	os.Exit(2)
}
