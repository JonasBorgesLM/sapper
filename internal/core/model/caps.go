package model

import (
	"errors"
	"fmt"
	"time"
)

// Caps are BlastGuard's mandatory blast-radius ceilings (SR-03): the maximum
// number of concurrent in-flight requests and the maximum wall-clock duration
// of a run. Both are required and must be positive — there is no safe silent
// default for either, so a zero value is a configuration error, not "unlimited".
type Caps struct {
	MaxConcurrency int           `json:"max_concurrency"`
	MaxDuration    time.Duration `json:"max_duration"`
}

// Validate reports whether both caps are present and positive. It names every
// offending field at once (not just the first) so one run surfaces the whole
// list rather than making the operator fix them one at a time.
func (c Caps) Validate() error {
	var errs []error
	if c.MaxConcurrency <= 0 {
		errs = append(errs, fmt.Errorf("max_concurrency must be a positive integer, got %d", c.MaxConcurrency))
	}
	if c.MaxDuration <= 0 {
		errs = append(errs, fmt.Errorf("max_duration must be a positive duration, got %s", c.MaxDuration))
	}
	return errors.Join(errs...)
}
