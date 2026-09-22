// Package probe samples an optional numeric metric the target exposes over HTTP,
// so a run can correlate a target-side value (memory, goroutines, queue depth)
// with the load over time. It reads a JSON object and extracts one dot-path
// numeric field (works with Go's expvar and most JSON /metrics endpoints).
//
// This is out-of-band monitoring, not load: it is low-frequency (once per
// window) and does not go through BlastGuard, so it never counts against the
// blast-radius caps.
package probe

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Sample fetches url and returns the numeric value at the dot-path field.
func Sample(url, field string) (float64, error) {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url) // #nosec G107 -- operator-supplied metrics URL, out-of-band monitoring
	if err != nil {
		return 0, fmt.Errorf("probe: get %s: %w", url, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return 0, fmt.Errorf("probe: read %s: %w", url, err)
	}
	return extractField(body, field)
}

// extractField parses data as a JSON object and returns the number at dotPath
// (e.g. "memstats.Alloc").
func extractField(data []byte, dotPath string) (float64, error) {
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return 0, fmt.Errorf("probe: parse JSON: %w", err)
	}
	var cur any = root
	for _, p := range strings.Split(dotPath, ".") {
		obj, ok := cur.(map[string]any)
		if !ok {
			return 0, fmt.Errorf("probe: %q: %q is not an object", dotPath, p)
		}
		cur, ok = obj[p]
		if !ok {
			return 0, fmt.Errorf("probe: %q: field %q not found", dotPath, p)
		}
	}
	f, ok := cur.(float64) // JSON numbers decode to float64
	if !ok {
		return 0, fmt.Errorf("probe: %q is not a number (%T)", dotPath, cur)
	}
	return f, nil
}
