package trustmodel

import (
	"sync"
	"time"

	"github.com/vs-uulm/go-taf/cmd/flags"
)

var simulationTime = struct {
	sync.RWMutex
	ns int64
}{}

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
}

func CreateListener(ttlSeconds int, checkIntervalSeconds int) EntityObserver {
	listener := EntityObserver{
		nodes:     make(map[string]int64),
		observers: make(map[observer]bool),
		lock:      &sync.RWMutex{},
		ttl:       ttlSeconds,
	}
	listener.startExpiryLoop(time.Duration(checkIntervalSeconds) * time.Second)
	return listener
}

func (l *EntityObserver) startExpiryLoop(checkInterval time.Duration) {
	go func() {
		for range time.Tick(checkInterval) {
			l.lock.Lock()
			if flags.SIMULATION && simulationTimeValue() > 0 {
				latest := simulationTimeValue()
				for key, ts := range l.nodes {
					if latest > ts+int64(l.ttl)*1e9 {
						delete(l.nodes, key)
						l.notifyObserversOnNodeRemoved(key)
					}
				}
			} else {
				now := time.Now().Unix()
				for key, ts := range l.nodes {
					if now > ts+int64(l.ttl) {
						delete(l.nodes, key)
						l.notifyObserversOnNodeRemoved(key)
					}
				}
			}
			l.lock.Unlock()
		}
	}()
}

func simulationTimeValue() int64 {
	simulationTime.RLock()
	defer simulationTime.RUnlock()
	return simulationTime.ns
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

func (l *EntityObserver) AddNode(identifier string) {
	l.lock.Lock()
	defer l.lock.Unlock()

	_, exists := l.nodes[identifier]
	l.nodes[identifier] = time.Now().Unix()
	if !exists {
		l.notifyObserversOnNodeAdded(identifier)
	}
}

func (l *EntityObserver) AddNodeAt(identifier string, timestampNs int64) {
	l.lock.Lock()
	defer l.lock.Unlock()

	if flags.SIMULATION {
		simulationTime.Lock()
		if timestampNs > simulationTime.ns {
			simulationTime.ns = timestampNs
		}
		simulationTime.Unlock()
	}

	_, exists := l.nodes[identifier]
	l.nodes[identifier] = timestampNs
	if !exists {
		l.notifyObserversOnNodeAdded(identifier)
	}
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
