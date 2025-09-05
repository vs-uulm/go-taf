package core

import (
	"context"
	"github.com/vs-uulm/go-taf/pkg/config"
	"log/slog"
)

/*
The TafContext struct captures several relevant properties needed by different subcomponents.
*/
type TafContext struct {
	Configuration config.Configuration
	Logger        *slog.Logger
	Context       context.Context
	Identifier    string
}
