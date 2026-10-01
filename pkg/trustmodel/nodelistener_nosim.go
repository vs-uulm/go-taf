//go:build !simulation
// +build !simulation

package trustmodel

import (
	"time"
)

// expiryState is not needed when expiry is checked by a wall-clock ticker.
type expiryState struct{}

/*
startExpiryLoop periodically removes nodes that have not been refreshed within the TTL, using the wall clock.
*/
func (l *EntityObserver) startExpiryLoop(checkIntervalSeconds int) {
	checkInterval := time.Duration(checkIntervalSeconds) * time.Second
	go func() {
		for range time.Tick(checkInterval) {
			l.lock.Lock()
			now := time.Now().Unix()
			for key, ts := range l.nodes {
				if now > ts+int64(l.ttl) {
					delete(l.nodes, key)
					l.notifyObserversOnNodeRemoved(key)
				}
			}
			l.lock.Unlock()
		}
	}()
}

// currentTimestamp returns the wall-clock time in seconds.
func currentTimestamp() int64 {
	return time.Now().Unix()
}

// advanceSimulationTime does nothing without simulation.
func advanceSimulationTime(int64) {}
