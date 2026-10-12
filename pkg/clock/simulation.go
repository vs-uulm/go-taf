//go:build simulation
// +build simulation

package clock

import (
	"sync"
)

// Source names the time returned by Now.
const Source = "simulation"

var simulationTime = struct {
	sync.Mutex
	ns    int64
	hooks []func(nowNs int64)
}{}

/*
Now returns the simulated time in nanoseconds, i.e., the latest time passed to Advance, or 0 before the first call.
*/
func Now() int64 {
	simulationTime.Lock()
	defer simulationTime.Unlock()
	return simulationTime.ns
}

/*
Advance sets the simulated time to the given time in nanoseconds, if it is later than the current one, and then calls
all hooks registered with OnAdvance (without holding the clock's lock).
*/
func Advance(timestampNs int64) {
	simulationTime.Lock()
	if timestampNs <= simulationTime.ns {
		simulationTime.Unlock()
		return
	}
	simulationTime.ns = timestampNs
	hooks := append([]func(nowNs int64){}, simulationTime.hooks...)
	simulationTime.Unlock()

	for _, hook := range hooks {
		hook(timestampNs)
	}
}

/*
OnAdvance registers a hook that is called with the new time whenever the simulated time advances.
*/
func OnAdvance(hook func(nowNs int64)) {
	simulationTime.Lock()
	defer simulationTime.Unlock()
	simulationTime.hooks = append(simulationTime.hooks, hook)
}
