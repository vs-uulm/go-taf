//go:build simulation
// +build simulation

package core

import (
	"context"
	"log/slog"
)

// SettlementTracker counts runnable work created by the platform. A simulation
// barrier waits for it to become idle after ordered ingress has been stopped.
type SettlementTracker struct {
	pending int
	total   int
	err     error
	done    chan struct{}
	mu      chan struct{}
	logger  *slog.Logger
}

func NewSettlementTracker(logger *slog.Logger) *SettlementTracker {
	done := make(chan struct{})
	close(done)
	if logger == nil {
		logger = slog.Default()
	}
	return &SettlementTracker{
		done:   done,
		mu:     make(chan struct{}, 1),
		logger: logger.With("Component", "Settlement"),
	}
}

func (t *SettlementTracker) Add() {
	if t == nil {
		return
	}
	t.mu <- struct{}{}
	if t.pending == 0 {
		t.total = 0
		t.err = nil
		t.resetDoneLocked()
	}
	t.pending++
	t.total++
	<-t.mu
}

func (t *SettlementTracker) Done(err error) {
	if t == nil {
		return
	}
	t.mu <- struct{}{}
	if err != nil && t.err == nil {
		t.err = err
	}
	if t.pending > 0 {
		t.pending--
		if t.pending == 0 {
			t.closeDoneLocked()
		}
	}
	<-t.mu
}

func (t *SettlementTracker) WaitIdle(ctx context.Context) error {
	if t == nil {
		return nil
	}
	t.mu <- struct{}{}
	done := t.done
	err := t.err
	pending := t.pending
	<-t.mu
	if pending == 0 {
		return err
	}

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return context.Cause(ctx)
	}
}

func (t *SettlementTracker) closeDoneLocked() {
	close(t.done)
}

func (t *SettlementTracker) resetDoneLocked() {
	t.done = make(chan struct{})
}
