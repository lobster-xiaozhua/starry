// Package bootcheck records startup self-check results for the product console.
//
// Design: the console shows real traffic data, not synthetic health checks.
// Boot-time self-checks are recorded once at startup; after that, all
// monitoring is based on actual request data from TrafficStats.
//
// This package deliberately has no imports from modules/* to avoid import cycles.
// main.go builds the ModuleStatus list and passes it in.
package bootcheck

import "time"

// Check represents a single boot-time self-check result.
type Check struct {
	Name   string `json:"name"`
	Status string `json:"status"` // ok | warn | fail
	Detail string `json:"detail,omitempty"`
	Count  int    `json:"count,omitempty"`
}

// ModuleStatus represents a business module's enabled/disabled state.
type ModuleStatus struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

// Result is the complete boot-time check result.
type Result struct {
	Timestamp time.Time      `json:"timestamp"`
	AppEnv    string         `json:"appEnv"`
	Version   string         `json:"version"`
	Checks    []Check        `json:"checks"`
	Modules   []ModuleStatus `json:"modules"`
}

var current *Result

// Record stores the boot-time check result.
func Record(r *Result) {
	current = r
}

// Get returns the recorded boot-time check result.
func Get() *Result {
	return current
}
