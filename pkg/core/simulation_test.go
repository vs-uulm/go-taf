//go:build simulation

package core

import (
	"context"
	"testing"
	"time"
)

func TestSettlementTrackerWaitIdleWhenInitiallyIdle(t *testing.T) {
	tracker := NewSettlementTracker(nil)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := tracker.WaitIdle(ctx); err != nil {
		t.Fatalf("WaitIdle returned an error for an idle tracker: %v", err)
	}
}
