package logging

import (
	"github.com/open-component-model/ocm-controller/internal/wasm/hostfuncs/types"
	"golang.org/x/exp/slog"
)

type handlerFunc func(*slog.Logger) types.HostFunc

var handlers = make(map[string]handlerFunc)

// GetRegistry returns the registered handlers.
func GetHandlers() map[string]handlerFunc {
	return handlers
}

func register(name string, f handlerFunc) {
	handlers[name] = f
}
