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

	// Windowing (optional): when a clock and window size are configured, each
	// record is also tallied into a per-time-window bucket, so a run can show a
	// timeline (when 429s began, when latency shifted) rather than only an
	// aggregate.
	now        func() time.Time
	windowSize time.Duration
	start      time.Time
	windows    []windowTally
}

type windowTally struct {
	total, errors int
	statusCounts  map[int]int
}

// Option configures a Collector at construction.
type Option func(*Collector)

// WithWindows tallies each record into a per-window bucket of the given size,
// timed by now, so Snapshot carries a timeline. now defaults off (no windowing).
func WithWindows(now func() time.Time, size time.Duration) Option {
	return func(c *Collector) {
		c.now = now
		c.windowSize = size
	}
}

// New returns an empty Collector. With WithWindows it also records a timeline.
func New(opts ...Option) *Collector {
	c := &Collector{statusCounts: make(map[int]int)}
	for _, opt := range opts {
		opt(c)
	}
	if c.now != nil {
		c.start = c.now()
	}
	return c
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
	c.windows = nil
	if c.now != nil {
		c.start = c.now()
	}
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
	failed := err != nil
	if failed {
		c.errors++
	} else {
		c.statusCounts[status]++
	}
	c.recordWindow(status, failed)
}

// recordWindow tallies one record into its time bucket. Caller holds c.mu.
func (c *Collector) recordWindow(status int, failed bool) {
	if c.now == nil {
		return
	}
	idx := int(c.now().Sub(c.start) / c.windowSize)
	if idx < 0 {
		idx = 0
	}
	for len(c.windows) <= idx {
		c.windows = append(c.windows, windowTally{statusCounts: make(map[int]int)})
	}
	w := &c.windows[idx]
	w.total++
	if failed {
		w.errors++
	} else {
		w.statusCounts[status]++
	}
}

// Snapshot is the aggregated view of everything recorded so far.
type Snapshot struct {
	Total        int          `json:"total"`
	Errors       int          `json:"errors"`
	ErrorRate    float64      `json:"error_rate"`
	StatusCounts map[int]int  `json:"status_counts"`
	Latency      LatencyStats `json:"latency"`
	// Windows is the per-time-window timeline, present only when the collector
	// was built WithWindows.
	Windows []WindowStat `json:"windows,omitempty"`
}

// WindowStat is one time window's tally: its start offset from the run start,
// how many requests fell in it, how many failed at the transport level, and the
// per-status breakdown.
type WindowStat struct {
	Start        time.Duration `json:"start"`
	Total        int           `json:"total"`
	Errors       int           `json:"errors"`
	StatusCounts map[int]int   `json:"status_counts"`
}

// LatencyStats summarises the recorded latencies. Percentiles use the
// nearest-rank method on the sorted samples.
//
// The latencies cover every request, including those that failed at the
// transport level: a timeout contributes its full wait, but a refused
// connection contributes a near-zero time that pulls the percentiles down. So
// these describe time-to-outcome, not time-to-success — read them alongside the
// error rate, which is why the two are reported together.
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

	for i, w := range c.windows {
		s.Windows = append(s.Windows, WindowStat{
			Start:        time.Duration(i) * c.windowSize,
			Total:        w.total,
			Errors:       w.errors,
			StatusCounts: maps.Clone(w.statusCounts),
		})
	}
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
