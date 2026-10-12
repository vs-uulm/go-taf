//go:build !simulation
// +build !simulation

package clock

import (
	"time"
)

// Source names the time returned by Now.
const Source = "wallclock"

// Now returns the wall-clock time as Unix time in nanoseconds.
func Now() int64 {
	return time.Now().UnixNano()
}

// Advance does nothing, as the wall clock advances by itself.
func Advance(int64) {}

// OnAdvance does nothing, as the hooks are never called without simulation.
func OnAdvance(func(nowNs int64)) {}
