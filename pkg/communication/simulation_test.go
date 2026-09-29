//go:build simulation

package communication

import (
	"context"
	"testing"
	"time"

	"github.com/vs-uulm/go-taf/pkg/core"
)

type simulationTestCommand struct{}

func (simulationTestCommand) Type() core.CommandType {
	return core.UNDEFINED
}

func TestDispatchToTAMWaitsForSimulation(t *testing.T) {
	tracker := core.NewSettlementTracker(nil)
	tracker.Add()
	tamChannel := make(chan core.Command, 1)
	ch := CommunicationInterface{
		tafContext: core.TafContext{
			Context:    context.Background(),
			Settlement: tracker,
		},
		channels: core.TafChannels{TAMChannel: tamChannel},
	}

	dispatched := make(chan struct{})
	go func() {
		ch.dispatchToTAM(simulationTestCommand{}, true)
		close(dispatched)
	}()

	select {
	case <-tamChannel:
		t.Fatal("command dispatched before prior work settled")
	case <-time.After(10 * time.Millisecond):
	}

	tracker.Done(nil)
	select {
	case <-tamChannel:
	case <-time.After(time.Second):
		t.Fatal("command was not dispatched after settlement")
	}
	<-dispatched
}
