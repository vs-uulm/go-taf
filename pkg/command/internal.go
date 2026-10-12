package command

import (
	"github.com/vs-uulm/go-taf/pkg/clock"
	"github.com/vs-uulm/go-taf/pkg/core"
)

/*
HandleTMIUpdate contains 1 or more update operations to be applied on a specified Trust Model Instance.
In case of more update operations, a worker should apply micro-batching and apply all updates before proceeding (e.g., calling the TLEE).
Time is the time of the updates in nanoseconds (see package clock), taken when the command is created, i.e., when the
TAF handles the message causing the updates, as workers may apply them later.
*/
type HandleTMIUpdate struct {
	commandType core.CommandType
	FullTmiID   string
	Updates     []core.Update
	Tag         *string
	Time        int64
}

func CreateHandleTMIUpdate(FullTmiID string, tag *string, updates ...core.Update) HandleTMIUpdate {
	return HandleTMIUpdate{
		FullTmiID:   FullTmiID,
		Updates:     updates,
		commandType: core.HANDLE_TMI_UPDATE,
		Tag:         tag,
		Time:        clock.Now(),
	}
}

func (r HandleTMIUpdate) Type() core.CommandType {
	return r.commandType
}

/*
HandleTMIInit is a command that initiates the existence of a Trust Model Instance for a TAM worker.
Time is the time of the spawn in nanoseconds (see package clock), taken when the command is created.
*/
type HandleTMIInit struct {
	commandType core.CommandType
	FullTmiID   string
	TMI         core.TrustModelInstance
	Time        int64
}

func CreateHandleTMIInit(fullTMIid string, TMI core.TrustModelInstance) HandleTMIInit {
	return HandleTMIInit{

		TMI:         TMI,
		FullTmiID:   fullTMIid,
		commandType: core.HANDLE_TMI_INIT,
		Time:        clock.Now(),
	}
}

func (r HandleTMIInit) Type() core.CommandType {
	return r.commandType
}

/*
HandleTMIDestroy is a command that signals a TAM worker to destroy a Trust Model Instance from its shard.
*/
type HandleTMIDestroy struct {
	commandType core.CommandType
	FullTmiID   string
}

func CreateHandleTMIDestroy(fullTMIid string) HandleTMIDestroy {
	return HandleTMIDestroy{
		FullTmiID:   fullTMIid,
		commandType: core.HANDLE_TMI_DESTROY,
	}
}

func (r HandleTMIDestroy) Type() core.CommandType {
	return r.commandType
}

/*
HandleATLUpdate is a command sent from a TAM worker to the TAM that contains new ATL results to be cached by the TAM.
*/

type HandleATLUpdate struct {
	commandType core.CommandType
	FullTmiID   string
	ResultSet   core.AtlResultSet
	Tag         *string
}

func CreateHandleATLUpdate(atl core.AtlResultSet, tag *string, fullTMI string) HandleATLUpdate {
	return HandleATLUpdate{
		ResultSet:   atl,
		FullTmiID:   fullTMI,
		commandType: core.HANDLE_ATL_UPDATE,
		Tag:         tag,
	}
}

func (r HandleATLUpdate) Type() core.CommandType {
	return r.commandType
}

type ObserverEvent uint16

const (
	EVENT_NODE_ADDED ObserverEvent = iota
	EVENT_NODE_REMOVED
)

type ObserverSource uint16

const (
	SOURCE_V2X ObserverSource = iota
	SOURCE_TCH
)

type HandleObserverEvent struct {
	commandType core.CommandType
	Identifier  string
	Event       ObserverEvent
	Source      ObserverSource
}

func CreateHandleObserverEvent(identifier string, event ObserverEvent, source ObserverSource) HandleObserverEvent {
	return HandleObserverEvent{
		commandType: core.HANDLE_OBSERVER_EVENT,
		Identifier:  identifier,
		Event:       event,
		Source:      source,
	}
}
func (r HandleObserverEvent) Type() core.CommandType {
	return r.commandType
}
