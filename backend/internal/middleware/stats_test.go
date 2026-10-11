package middleware

import (
	"math"
	"sync/atomic"
	"testing"
	"time"
)

func newTestTrafficStats() *TrafficStats {
	return &TrafficStats{
		startedAt: time.Now(),
		byPath:    make(map[string]*pathStat),
		byStatus:  make(map[string]*atomic.Int64),
	}
}

func TestTrafficStatsSnapshot(t *testing.T) {
	ts := newTestTrafficStats()
	ts.Record("/api/a", "GET", 200, 10*time.Millisecond)
	ts.Record("/api/a", "GET", 404, 30*time.Millisecond)
	ts.Record("/api/b", "POST", 500, 20*time.Millisecond)

	snap := ts.Snapshot()
	if snap.TotalRequests != 3 {
		t.Fatalf("TotalRequests = %d, want 3", snap.TotalRequests)
	}
	if snap.TotalErrors != 2 {
		t.Fatalf("TotalErrors = %d, want 2", snap.TotalErrors)
	}
	if want := 2.0 / 3.0; math.Abs(snap.ErrorRate-want) > 1e-9 {
		t.Fatalf("ErrorRate = %v, want %v", snap.ErrorRate, want)
	}
	// (10+30+20)/3 = 20ms
	if math.Abs(snap.AvgLatencyMs-20) > 1e-6 {
		t.Fatalf("AvgLatencyMs = %v, want 20", snap.AvgLatencyMs)
	}

	byPath := make(map[string]PathSnapshot, len(snap.ByPath))
	for _, p := range snap.ByPath {
		byPath[p.Path] = p
	}
	a, ok := byPath["/api/a"]
	if !ok {
		t.Fatalf("missing path stat for /api/a: %+v", snap.ByPath)
	}
	if a.Count != 2 {
		t.Fatalf("/api/a count = %d, want 2", a.Count)
	}
	if math.Abs(a.AvgLatencyMs-20) > 1e-6 {
		t.Fatalf("/api/a avg = %v, want 20", a.AvgLatencyMs)
	}
	if math.Abs(a.ErrorRate-0.5) > 1e-9 {
		t.Fatalf("/api/a error rate = %v, want 0.5", a.ErrorRate)
	}

	byStatus := make(map[string]int64, len(snap.ByStatus))
	for _, s := range snap.ByStatus {
		byStatus[s.Status] = s.Count
	}
	for status, want := range map[string]int64{"2xx": 1, "4xx": 1, "5xx": 1} {
		if byStatus[status] != want {
			t.Fatalf("status %s = %d, want %d (all: %+v)", status, byStatus[status], want, snap.ByStatus)
		}
	}
}

func TestTrafficStatsEmptySnapshot(t *testing.T) {
	snap := newTestTrafficStats().Snapshot()
	if snap.TotalRequests != 0 || snap.TotalErrors != 0 || snap.ErrorRate != 0 || snap.AvgLatencyMs != 0 {
		t.Fatalf("empty snapshot should be zeroed, got %+v", snap)
	}
	if len(snap.ByPath) != 0 || len(snap.ByStatus) != 0 {
		t.Fatalf("empty snapshot should have no breakdowns, got %+v", snap)
	}
}

func TestStatusClass(t *testing.T) {
	cases := []struct {
		code int
		want string
	}{
		{100, "1xx"},
		{200, "2xx"},
		{204, "2xx"},
		{301, "3xx"},
		{404, "4xx"},
		{429, "4xx"},
		{500, "5xx"},
		{503, "5xx"},
	}
	for _, tc := range cases {
		if got := statusClass(tc.code); got != tc.want {
			t.Errorf("statusClass(%d) = %q, want %q", tc.code, got, tc.want)
		}
	}
}
