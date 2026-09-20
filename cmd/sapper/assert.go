package main

import (
	"flag"
	"fmt"
)

// runAssert implements `sapper assert`: it reads a result file and gates on the
// stored verdict, exiting non-zero when any SLO is red so CI can gate on it.
func runAssert(args []string) error {
	fs := flag.NewFlagSet("assert", flag.ContinueOnError)
	in := fs.String("in", "result.json", "path to the result JSON to assert")
	if err := fs.Parse(args); err != nil {
		return err
	}

	result, err := readResult(*in)
	if err != nil {
		return err
	}

	for _, r := range result.Verdict.Results {
		status := "ok"
		if !r.Passed {
			status = "FAIL"
		}
		fmt.Printf("  [%s] %s: %s\n", status, r.Name, r.Detail)
	}

	if !result.Verdict.Passed {
		return fmt.Errorf("one or more SLOs failed")
	}
	fmt.Println("all SLOs passed")
	return nil
}
