package web

import (
	"sync"

	"github.com/vs-uulm/go-taf/pkg/listener"
)

/*
This file contains all listener functions that will be called by the core TAF whenever a certain event occurs.
To decouple processing from the core TAF, the event is put into an unbounded queue and then handled from the
web server go-routine, separated from the core TAF. Pushing to the queue never blocks the TAF and never drops events.
*/

/*
eventQueue is an unbounded FIFO queue of listener events. Producers (the TAF) append events without blocking, the single
consumer (State.Handle) is notified via signal and drains all queued events at once.
*/
type eventQueue struct {
	mutex  sync.Mutex
	events []listener.ListenerEvent
	signal chan struct{}
}

func newEventQueue() *eventQueue {
	return &eventQueue{
		events: make([]listener.ListenerEvent, 0),
		signal: make(chan struct{}, 1),
	}
}

func (q *eventQueue) push(event listener.ListenerEvent) {
	q.mutex.Lock()
	q.events = append(q.events, event)
	q.mutex.Unlock()

	// notify the consumer, unless a notification is already pending
	select {
	case q.signal <- struct{}{}:
	default:
	}
}

func (q *eventQueue) drain() []listener.ListenerEvent {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	events := q.events
	q.events = make([]listener.ListenerEvent, 0, len(events))
	return events
}

func (s *Webserver) OnSessionCreated(event listener.SessionCreatedEvent) {
	s.events.push(event)
}

func (s *Webserver) OnSessionTorndown(event listener.SessionTorndownEvent) {
	s.events.push(event)
}

func (s *Webserver) OnATLUpdated(event listener.ATLUpdatedEvent) {
	s.events.push(event)
}

func (s *Webserver) OnATLRemoved(event listener.ATLRemovedEvent) {
	s.events.push(event)
}

func (s *Webserver) OnTrustModelInstanceSpawned(event listener.TrustModelInstanceSpawnedEvent) {
	s.events.push(event)
}

func (s *Webserver) OnTrustModelInstanceUpdated(event listener.TrustModelInstanceUpdatedEvent) {
	s.events.push(event)
}

func (s *Webserver) OnTrustModelInstanceDeleted(event listener.TrustModelInstanceDeletedEvent) {
	s.events.push(event)
}
