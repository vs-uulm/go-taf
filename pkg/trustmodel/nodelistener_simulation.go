//go:build simulation
// +build simulation

package trustmodel

import (
	"sync"
)

/*
simulationTime is the latest simulated time in nanoseconds, as advanced by AddNodeAt (i.e., by the reference times of
V2X CAMs). It is shared by all listeners, as only the V2X listener receives timestamped messages.
*/
var simulationTime = struct {
	sync.RWMutex
	ns int64
}{}

// simulationListeners are all listeners whose expiry is checked whenever the simulated time advances.
var simulationListeners = struct {
	sync.Mutex
	listeners []*EntityObserver
}{}

// expiryState holds the state of the expiry check driven by the simulated time, guarded by the listener's lock.
type expiryState struct {
	checkIntervalNs int64
	lastCheckNs     int64
}

/*
startExpiryLoop registers the listener for expiry checks driven by the simulated time instead of starting a wall-clock
ticker: whenever the simulated time advances by at least the check interval, nodes that have not been refreshed within
the TTL (in simulated time) are removed.
*/
func (l *EntityObserver) startExpiryLoop(checkIntervalSeconds int) {
	l.expiry = &expiryState{checkIntervalNs: int64(checkIntervalSeconds) * 1e9}
	simulationListeners.Lock()
	defer simulationListeners.Unlock()
	simulationListeners.listeners = append(simulationListeners.listeners, l)
}

/*
currentTimestamp returns the simulated time in nanoseconds. Before the first V2X CAM, the simulated time is not known
yet and 0 is returned; such nodes are timestamped with the first known simulated time (see expireAt).
*/
func currentTimestamp() int64 {
	return simulationTimeValue()
}

func simulationTimeValue() int64 {
	simulationTime.RLock()
	defer simulationTime.RUnlock()
	return simulationTime.ns
}

/*
advanceSimulationTime sets the simulated time to the given timestamp, if newer, and then checks all listeners for
expired nodes.
*/
func advanceSimulationTime(timestampNs int64) {
	simulationTime.Lock()
	if timestampNs <= simulationTime.ns {
		simulationTime.Unlock()
		return
	}
	simulationTime.ns = timestampNs
	simulationTime.Unlock()

	simulationListeners.Lock()
	listeners := append([]*EntityObserver(nil), simulationListeners.listeners...)
	simulationListeners.Unlock()

	for _, listener := range listeners {
		listener.expireAt(timestampNs)
	}
}

/*
expireAt removes nodes that have not been refreshed within the TTL, if at least the check interval of simulated time
has passed since the last check. Nodes added before the simulated time was known (timestamp 0) are timestamped with
the current simulated time instead, so that they are not removed right away when the first CAM sets the simulated time.
*/
func (l *EntityObserver) expireAt(nowNs int64) {
	l.lock.Lock()
	defer l.lock.Unlock()

	if nowNs < l.expiry.lastCheckNs+l.expiry.checkIntervalNs {
		return
	}
	l.expiry.lastCheckNs = nowNs
	for key, ts := range l.nodes {
		if ts == 0 {
			l.nodes[key] = nowNs
			continue
		}
		if nowNs > ts+int64(l.ttl)*1e9 {
			delete(l.nodes, key)
			l.notifyObserversOnNodeRemoved(key)
		}
	}
}
