//go:build !simulation
// +build !simulation

package core

import (
	"context"
	"log/slog"
)

// SettlementTracker is a no-op placeholder when settlement is disabled at compile time.
type SettlementTracker struct{}

func NewSettlementTracker(*slog.Logger) *SettlementTracker {
	return &SettlementTracker{}
}

func (t *SettlementTracker) Add()                           {}
func (t *SettlementTracker) Done(error)                     {}
func (t *SettlementTracker) WaitIdle(context.Context) error { return nil }
