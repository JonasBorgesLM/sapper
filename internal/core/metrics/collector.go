package metrics

import (
	"maps"
	"math"
	"slices"
	"sync"
	"time"
)

// Collector records the outcome of every request during a run: its latency, its
// HTTP status (when it got a response), and whether it failed at the transport
// level. It is safe for concurrent use — the generator records from many workers
// at once — and cheap per call (an append and a couple of counters), so it does
// not distort the measurement it takes (FR-06).
//
// Latencies are kept as samples and percentiles are computed exactly on
// Snapshot. This is simpler and more accurate than an approximate histogram for
// v1's bounded runs; the public API is percentile-based, so the storage can
// become a bounded histogram later (for long soaks) without changing callers.
type Collector struct {
	mu           sync.Mutex
	latencies    []time.Duration
	statusCounts map[int]int
	errors       int
	total        int
}

// New returns an empty Collector.
func New() *Collector {
	return &Collector{statusCounts: make(map[int]int)}
}

// Reset discards everything recorded so far. The generator calls it when the
// warm-up window ends, so warm-up samples are excluded from the reported
// statistics (FR-07).
func (c *Collector) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.latencies = nil
	c.statusCounts = make(map[int]int)
	c.errors = 0
	c.total = 0
}

// Record logs one request. latency is the time to its outcome. A non-nil err is
// a transport-level failure (connection refused, timeout) and counts toward the
// error rate; otherwise status is tallied. A 5xx is a status, not an error —
// SLOs assert on statuses separately from the transport error rate.
func (c *Collector) Record(latency time.Duration, status int, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.total++
	c.latencies = append(c.latencies, latency)
	if err != nil {
		c.errors++
		return
	}
	c.statusCounts[status]++
}

// Snapshot is the aggregated view of everything recorded so far.
type Snapshot struct {
	Total        int          `json:"total"`
	Errors       int          `json:"errors"`
	ErrorRate    float64      `json:"error_rate"`
	StatusCounts map[int]int  `json:"status_counts"`
	Latency      LatencyStats `json:"latency"`
}

// LatencyStats summarises the recorded latencies. Percentiles use the
// nearest-rank method on the sorted samples.
type LatencyStats struct {
	Count int           `json:"count"`
	Min   time.Duration `json:"min"`
	Max   time.Duration `json:"max"`
	Mean  time.Duration `json:"mean"`
	P50   time.Duration `json:"p50"`
	P90   time.Duration `json:"p90"`
	P99   time.Duration `json:"p99"`
}

// Snapshot computes the current aggregate. It copies and sorts the latencies, so
// it is O(n log n) and meant to be called at the end of a run (or a window), not
// per request.
func (c *Collector) Snapshot() Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()

	s := Snapshot{
		Total:        c.total,
		Errors:       c.errors,
		StatusCounts: maps.Clone(c.statusCounts),
	}
	if s.StatusCounts == nil {
		s.StatusCounts = make(map[int]int)
	}
	if c.total > 0 {
		s.ErrorRate = float64(c.errors) / float64(c.total)
	}

	sorted := slices.Clone(c.latencies)
	slices.Sort(sorted)
	s.Latency = latencyStats(sorted)
	return s
}

func latencyStats(sorted []time.Duration) LatencyStats {
	n := len(sorted)
	if n == 0 {
		return LatencyStats{}
	}
	var sum time.Duration
	for _, d := range sorted {
		sum += d
	}
	return LatencyStats{
		Count: n,
		Min:   sorted[0],
		Max:   sorted[n-1],
		Mean:  sum / time.Duration(n),
		P50:   percentile(sorted, 50),
		P90:   percentile(sorted, 90),
		P99:   percentile(sorted, 99),
	}
}

// percentile returns the nearest-rank pth percentile of a sorted slice: the
// smallest value at or below which p% of the samples fall.
func percentile(sorted []time.Duration, p float64) time.Duration {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	rank := int(math.Ceil(p/100*float64(n))) - 1
	if rank < 0 {
		rank = 0
	}
	if rank >= n {
		rank = n - 1
	}
	return sorted[rank]
}
