package middleware

import (
	"sync"
	"sync/atomic"
	"time"
)

// TrafficStats collects lightweight in-memory traffic statistics from real
// requests. Unlike Prometheus metrics (which are scraped externally), this
// struct powers the product console's health dashboard by providing a
// JSON-serializable summary of real user traffic.
//
// Design: the console shows real traffic data, not synthetic health checks.
// Boot-time self-checks are recorded once at startup; after that, all
// monitoring is based on actual request data flowing through this collector.
type TrafficStats struct {
	startedAt    time.Time
	totalReqs    atomic.Int64
	totalErrors  atomic.Int64
	totalLatency atomic.Int64 // sum of latency in microseconds

	mu       sync.RWMutex
	byPath   map[string]*pathStat
	byStatus map[string]*atomic.Int64
}

type pathStat struct {
	count   atomic.Int64
	errors  atomic.Int64
	latency atomic.Int64 // microseconds
}

var globalStats = &TrafficStats{
	startedAt: time.Now(),
	byPath:    make(map[string]*pathStat),
	byStatus:  make(map[string]*atomic.Int64),
}

// GetTrafficStats returns the global traffic stats collector.
func GetTrafficStats() *TrafficStats {
	return globalStats
}

// Record updates traffic stats for a single request.
func (ts *TrafficStats) Record(path, method string, statusCode int, latency time.Duration) {
	ts.totalReqs.Add(1)
	ts.totalLatency.Add(latency.Microseconds())
	if statusCode >= 400 {
		ts.totalErrors.Add(1)
	}

	statusKey := statusClass(statusCode)
	ts.mu.Lock()
	ps, ok := ts.byPath[path]
	if !ok {
		ps = &pathStat{}
		ts.byPath[path] = ps
	}
	sc, ok := ts.byStatus[statusKey]
	if !ok {
		sc = &atomic.Int64{}
		ts.byStatus[statusKey] = sc
	}
	ts.mu.Unlock()

	ps.count.Add(1)
	ps.latency.Add(latency.Microseconds())
	if statusCode >= 400 {
		ps.errors.Add(1)
	}
	sc.Add(1)
}

// Snapshot returns a JSON-serializable summary of current traffic stats.
func (ts *TrafficStats) Snapshot() TrafficSnapshot {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	total := ts.totalReqs.Load()
	totalLatencyUs := ts.totalLatency.Load()
	totalErrors := ts.totalErrors.Load()

	avgLatencyMs := 0.0
	if total > 0 {
		avgLatencyMs = float64(totalLatencyUs) / float64(total) / 1000.0
	}

	errorRate := 0.0
	if total > 0 {
		errorRate = float64(totalErrors) / float64(total)
	}

	paths := make([]PathSnapshot, 0, len(ts.byPath))
	for p, ps := range ts.byPath {
		count := ps.count.Load()
		latUs := ps.latency.Load()
		errs := ps.errors.Load()
		avgMs := 0.0
		if count > 0 {
			avgMs = float64(latUs) / float64(count) / 1000.0
		}
		errRate := 0.0
		if count > 0 {
			errRate = float64(errs) / float64(count)
		}
		paths = append(paths, PathSnapshot{
			Path:         p,
			Count:        count,
			AvgLatencyMs: avgMs,
			ErrorRate:    errRate,
		})
	}

	statuses := make([]StatusSnapshot, 0, len(ts.byStatus))
	for s, c := range ts.byStatus {
		statuses = append(statuses, StatusSnapshot{
			Status: s,
			Count:  c.Load(),
		})
	}

	return TrafficSnapshot{
		StartedAt:     ts.startedAt,
		TotalRequests: total,
		TotalErrors:   totalErrors,
		ErrorRate:     errorRate,
		AvgLatencyMs:  avgLatencyMs,
		UptimeSeconds: int64(time.Since(ts.startedAt).Seconds()),
		ByPath:        paths,
		ByStatus:      statuses,
	}
}

// TrafficSnapshot is the JSON-serializable traffic summary for the console.
type TrafficSnapshot struct {
	StartedAt     time.Time        `json:"startedAt"`
	TotalRequests int64            `json:"totalRequests"`
	TotalErrors   int64            `json:"totalErrors"`
	ErrorRate     float64          `json:"errorRate"`
	AvgLatencyMs  float64          `json:"avgLatencyMs"`
	UptimeSeconds int64            `json:"uptimeSeconds"`
	ByPath        []PathSnapshot   `json:"byPath"`
	ByStatus      []StatusSnapshot `json:"byStatus"`
}

// PathSnapshot is per-path traffic stats.
type PathSnapshot struct {
	Path         string  `json:"path"`
	Count        int64   `json:"count"`
	AvgLatencyMs float64 `json:"avgLatencyMs"`
	ErrorRate    float64 `json:"errorRate"`
}

// StatusSnapshot is per-status-class request count.
type StatusSnapshot struct {
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

func statusClass(code int) string {
	switch {
	case code >= 500:
		return "5xx"
	case code >= 400:
		return "4xx"
	case code >= 300:
		return "3xx"
	case code >= 200:
		return "2xx"
	default:
		return "1xx"
	}
}
