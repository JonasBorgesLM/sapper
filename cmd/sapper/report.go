package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/JonasBorgesLM/sapper/internal/core/report"
)

// runReport implements `sapper report`: it renders a result file to a
// self-contained HTML report. It never touches the network.
func runReport(args []string) error {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	in := fs.String("in", "result.json", "path to the result JSON")
	out := fs.String("out", "report.html", "path to write the HTML report")
	if err := fs.Parse(args); err != nil {
		return err
	}

	result, err := readResult(*in)
	if err != nil {
		return err
	}
	html, err := report.Render(result)
	if err != nil {
		return err
	}
	// The report is not sensitive and is meant to be opened and shared.
	if err := os.WriteFile(*out, []byte(html), 0o644); err != nil { // #nosec G306
		return fmt.Errorf("writing %s: %w", *out, err)
	}
	fmt.Printf("wrote report to %s\n", *out)
	return nil
}
