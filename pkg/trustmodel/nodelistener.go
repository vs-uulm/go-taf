package trustmodel

import (
	"sync"
)

/*
How nodes are timestamped and how their expiry is checked depends on the build:
  - without the simulation build tag (nodelistener_nosim.go), nodes are timestamped with the wall clock and a wall-clock
    ticker checks for expired nodes,
  - with the simulation build tag (nodelistener_simulation.go), nodes are timestamped with the simulated time and the
    expiry check is driven by the simulated time.
*/

type observer interface {
	handleNodeAdded(identifier string)
	handleNodeRemoved(identifier string)
}

/*
EntityObserver implements the observer pattern and provides an interface to register listeners to be called when new
entities have been added or removed.
*/
type EntityObserver struct {
	nodes     map[string]int64
	observers map[observer]bool
	lock      *sync.RWMutex
	ttl       int
	expiry    *expiryState
}

func CreateListener(ttlSeconds int, checkIntervalSeconds int) EntityObserver {
	listener := EntityObserver{
		nodes:     make(map[string]int64),
		observers: make(map[observer]bool),
		lock:      &sync.RWMutex{},
		ttl:       ttlSeconds,
	}
	listener.startExpiryLoop(checkIntervalSeconds)
	return listener
}

func (l *EntityObserver) registerObserver(observer observer) {
	l.observers[observer] = true
}

func (l *EntityObserver) removeObserver(observer observer) {
	delete(l.observers, observer)
}

func (l *EntityObserver) notifyObserversOnNodeAdded(identifier string) {
	for observer := range l.observers {
		observer.handleNodeAdded(identifier)
	}
}

func (l *EntityObserver) notifyObserversOnNodeRemoved(identifier string) {
	for observer := range l.observers {
		observer.handleNodeRemoved(identifier)
	}
}

/*
AddNode adds or refreshes a node with the current time (see currentTimestamp).
*/
func (l *EntityObserver) AddNode(identifier string) {
	l.lock.Lock()
	defer l.lock.Unlock()

	_, exists := l.nodes[identifier]
	l.nodes[identifier] = currentTimestamp()
	if !exists {
		l.notifyObserversOnNodeAdded(identifier)
	}
}

/*
AddNodeAt adds or refreshes a node with the given timestamp in nanoseconds. In simulation builds, the timestamp also
advances the simulated time.
*/
func (l *EntityObserver) AddNodeAt(identifier string, timestampNs int64) {
	l.lock.Lock()
	_, exists := l.nodes[identifier]
	l.nodes[identifier] = timestampNs
	if !exists {
		l.notifyObserversOnNodeAdded(identifier)
	}
	l.lock.Unlock()

	// called without holding the lock, as advancing the simulated time checks all listeners for expired nodes
	advanceSimulationTime(timestampNs)
}

func (l *EntityObserver) RemoveNode(identifier string) {
	l.lock.Lock()
	defer l.lock.Unlock()

	_, exists := l.nodes[identifier]
	if exists {
		delete(l.nodes, identifier)
		l.notifyObserversOnNodeRemoved(identifier)
	}
}

func (l *EntityObserver) Nodes() []string {
	l.lock.RLock()
	defer l.lock.RUnlock()

	nodes := make([]string, len(l.nodes))
	i := 0
	for node := range l.nodes {
		nodes[i] = node
		i++
	}
	return nodes
}
