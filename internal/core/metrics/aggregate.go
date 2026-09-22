package metrics

import (
	"math"
	"time"
)

// Aggregation summarises several runs of the same scenario. Honest reporting
// means never a single "best" run: each metric is reported as its mean and
// spread (population standard deviation) across the runs, plus the min and max
// observed, so a p99 that swings run to run cannot be hidden (ADR-0003, FR-07).
type Aggregation struct {
	Runs         int          `json:"runs"`
	StatusCounts map[int]int  `json:"status_counts"`
	P50          DurationStat `json:"p50"`
	P90          DurationStat `json:"p90"`
	P99          DurationStat `json:"p99"`
	ErrorRate    RateStat     `json:"error_rate"`
}

// DurationStat is the spread of a latency metric across runs.
type DurationStat struct {
	Mean   time.Duration `json:"mean"`
	Min    time.Duration `json:"min"`
	Max    time.Duration `json:"max"`
	StdDev time.Duration `json:"stddev"`
}

// RateStat is the spread of a rate metric across runs.
type RateStat struct {
	Mean   float64 `json:"mean"`
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	StdDev float64 `json:"stddev"`
}

// Aggregate combines N run snapshots. An empty input yields a zero Aggregation.
func Aggregate(snaps []Snapshot) Aggregation {
	n := len(snaps)
	if n == 0 {
		return Aggregation{}
	}
	p50 := make([]float64, n)
	p90 := make([]float64, n)
	p99 := make([]float64, n)
	rate := make([]float64, n)
	for i, s := range snaps {
		p50[i] = float64(s.Latency.P50)
		p90[i] = float64(s.Latency.P90)
		p99[i] = float64(s.Latency.P99)
		rate[i] = s.ErrorRate
	}
	statusCounts := make(map[int]int)
	for _, s := range snaps {
		for code, c := range s.StatusCounts {
			statusCounts[code] += c
		}
	}
	return Aggregation{
		Runs:         n,
		StatusCounts: statusCounts,
		P50:          durationStatOf(p50),
		P90:          durationStatOf(p90),
		P99:          durationStatOf(p99),
		ErrorRate:    rateStatOf(rate),
	}
}

// stats returns the mean, min, max and population standard deviation of xs,
// which must be non-empty.
func stats(xs []float64) (mean, min, max, stddev float64) {
	min, max = xs[0], xs[0]
	var sum float64
	for _, x := range xs {
		sum += x
		if x < min {
			min = x
		}
		if x > max {
			max = x
		}
	}
	mean = sum / float64(len(xs))
	var sq float64
	for _, x := range xs {
		d := x - mean
		sq += d * d
	}
	stddev = math.Sqrt(sq / float64(len(xs)))
	return mean, min, max, stddev
}

func durationStatOf(xs []float64) DurationStat {
	m, lo, hi, sd := stats(xs)
	return DurationStat{
		Mean:   time.Duration(m),
		Min:    time.Duration(lo),
		Max:    time.Duration(hi),
		StdDev: time.Duration(sd),
	}
}

func rateStatOf(xs []float64) RateStat {
	m, lo, hi, sd := stats(xs)
	return RateStat{Mean: m, Min: lo, Max: hi, StdDev: sd}
}

// Knee returns the request rate (req/s) at which a status first appears in the
// timeline — the point where, e.g., a rate limiter's 429 begins. rps is the
// cumulative throughput up to the end of that window; at is the elapsed time to
// it. ok is false if the status never appears or there are no windows.
func Knee(windows []WindowStat, windowSize time.Duration, status int) (rps float64, at time.Duration, ok bool) {
	cumulative := 0
	for _, w := range windows {
		cumulative += w.Total
		if w.StatusCounts[status] > 0 {
			end := w.Start + windowSize
			if end <= 0 {
				return 0, 0, false
			}
			return float64(cumulative) / end.Seconds(), end, true
		}
	}
	return 0, 0, false
}
