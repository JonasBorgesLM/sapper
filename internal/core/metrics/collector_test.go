package metrics

import (
	"errors"
	"sync"
	"testing"
	"time"
)

const ms = time.Millisecond

func TestSnapshotCountsStatusesAndErrorRate(t *testing.T) {
	c := New()
	c.Record(10*ms, 200, nil)
	c.Record(20*ms, 200, nil)
	c.Record(30*ms, 500, nil) // a 5xx is a status, not a transport error
	c.Record(5*ms, 0, errors.New("connection refused"))

	s := c.Snapshot()
	if s.Total != 4 {
		t.Errorf("Total = %d, want 4", s.Total)
	}
	if s.Errors != 1 {
		t.Errorf("Errors = %d, want 1 (only the transport error)", s.Errors)
	}
	if s.ErrorRate != 0.25 {
		t.Errorf("ErrorRate = %v, want 0.25", s.ErrorRate)
	}
	if s.StatusCounts[200] != 2 || s.StatusCounts[500] != 1 {
		t.Errorf("StatusCounts = %v, want {200:2, 500:1}", s.StatusCounts)
	}
}

func TestLatencyPercentilesNearestRank(t *testing.T) {
	c := New()
	for i := 1; i <= 10; i++ {
		c.Record(time.Duration(i*10)*ms, 200, nil) // 10ms..100ms
	}
	s := c.Snapshot().Latency
	if s.Min != 10*ms || s.Max != 100*ms {
		t.Errorf("Min/Max = %s/%s, want 10ms/100ms", s.Min, s.Max)
	}
	if s.P50 != 50*ms {
		t.Errorf("P50 = %s, want 50ms", s.P50)
	}
	if s.P90 != 90*ms {
		t.Errorf("P90 = %s, want 90ms", s.P90)
	}
	if s.P99 != 100*ms {
		t.Errorf("P99 = %s, want 100ms", s.P99)
	}
}

func TestConcurrentRecordIsRaceFree(t *testing.T) {
	c := New()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Record(ms, 200, nil)
		}()
	}
	wg.Wait()
	if got := c.Snapshot().Total; got != 100 {
		t.Errorf("Total = %d, want 100", got)
	}
}

func TestWindowedRecording(t *testing.T) {
	now := time.Unix(0, 0)
	clock := func() time.Time { return now }
	c := New(WithWindows(clock, 100*ms))

	c.Record(ms, 200, nil) // window 0
	c.Record(ms, 429, nil) // window 0
	now = now.Add(150 * ms)
	c.Record(ms, 429, nil) // window 1

	s := c.Snapshot()
	if len(s.Windows) != 2 {
		t.Fatalf("len(Windows) = %d, want 2", len(s.Windows))
	}
	if s.Windows[0].Total != 2 || s.Windows[0].StatusCounts[200] != 1 || s.Windows[0].StatusCounts[429] != 1 {
		t.Errorf("window 0 = %+v", s.Windows[0])
	}
	if s.Windows[1].Total != 1 || s.Windows[1].StatusCounts[429] != 1 {
		t.Errorf("window 1 = %+v", s.Windows[1])
	}
	if s.Windows[1].Start != 100*ms {
		t.Errorf("window 1 Start = %s, want 100ms", s.Windows[1].Start)
	}
}

func TestNonWindowedHasNoWindows(t *testing.T) {
	c := New()
	c.Record(ms, 200, nil)
	if w := c.Snapshot().Windows; len(w) != 0 {
		t.Errorf("Windows = %v, want none when windowing is off", w)
	}
}
