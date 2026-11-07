package scenario

import (
	"crypto/rand"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Per-request template variables (ADR-0010).
//
// A scenario's path, header values and body may contain {{uuid}} and {{seq}},
// filled in again for every request. They are deliberately a different syntax
// from the ${VAR} environment expansion: ${VAR} is resolved once, at load, from
// the operator's environment (a secret); {{...}} is resolved per request from
// the run itself, so a load test of a write endpoint can send a distinct
// idempotency key or id each time instead of replaying one request.
//
//	{{uuid}}  a fresh random (v4) UUID. Every occurrence within one request is
//	          the same value, so one request can carry it in several places;
//	          the next request gets another.
//	{{seq}}   a counter, 1 for the first request, shared by all workers and
//	          phases of the run.

var placeholder = regexp.MustCompile(`\{\{([^{}]*)\}\}`)

// Vars are the values substituted into one request.
type Vars struct {
	UUID string
	Seq  uint64
}

// NewUUID returns a random RFC 4122 version 4 UUID.
func NewUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand does not fail on supported platforms; a load generator that
		// silently reused an id would invalidate the run it exists to perform.
		panic(fmt.Sprintf("scenario: crypto/rand: %v", err))
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// Render substitutes v into every placeholder of s. Validation at load has
// already refused unknown names, so one reaching here is left as it is.
func Render(s string, v Vars) string {
	if !strings.Contains(s, "{{") {
		return s
	}
	return placeholder.ReplaceAllStringFunc(s, func(m string) string {
		switch strings.TrimSpace(m[2 : len(m)-2]) {
		case "uuid":
			return v.UUID
		case "seq":
			return strconv.FormatUint(v.Seq, 10)
		}
		return m
	})
}

// checkPlaceholders reports every placeholder in s that is not a known
// variable. A typo such as {{uid}} must fail the load: sent literally, it
// would turn a run of distinct requests into one repeated request that the
// target deduplicates, and the run would measure the wrong thing while
// looking green.
func checkPlaceholders(where, s string) []error {
	var errs []error
	for _, m := range placeholder.FindAllStringSubmatch(s, -1) {
		switch strings.TrimSpace(m[1]) {
		case "uuid", "seq":
		default:
			errs = append(errs, fmt.Errorf("%s: unknown template variable {{%s}} (want {{uuid}} or {{seq}})", where, m[1]))
		}
	}
	return errs
}
