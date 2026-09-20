// Command sapper is the resilience test-harness CLI. Its subcommands are
// independent pipeline stages chained through a result file on disk:
//
//	sapper run    --scenario s.yaml --config c.yaml --out result.json
//	sapper assert --in result.json                 # exit != 0 on a red SLO (CI gate)
//	sapper report --in result.json --out report.html
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/JonasBorgesLM/sapper/internal/core/model"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	// SIGINT/SIGTERM cancels ctx, which the run wires to BlastGuard's kill
	// switch: an immediate halt of new load with a graceful drain (SR-04).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var err error
	switch os.Args[1] {
	case "run":
		err = runRun(ctx, os.Args[2:])
	case "assert":
		err = runAssert(os.Args[2:])
	case "report":
		err = runReport(os.Args[2:])
	case "-h", "--help", "help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "sapper: unknown subcommand %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "sapper:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `sapper — resilience test harness (authorized lab/staging targets only)

Usage:
  sapper run    --config c.yaml --scenario s.yaml [--out result.json] [--runs 3]
  sapper assert --in result.json
  sapper report --in result.json [--out report.html]
`)
}

// readResult loads a result file written by `run`, shared by assert and report.
func readResult(path string) (model.Result, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- operator-supplied --in path
	if err != nil {
		return model.Result{}, fmt.Errorf("reading %s: %w", path, err)
	}
	var r model.Result
	if err := json.Unmarshal(data, &r); err != nil {
		return model.Result{}, fmt.Errorf("parsing %s: %w", path, err)
	}
	return r, nil
}
