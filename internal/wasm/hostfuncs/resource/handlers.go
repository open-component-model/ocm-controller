package resource

import (
	"github.com/open-component-model/ocm-controller/internal/wasm/hostfuncs/types"
	"github.com/open-component-model/ocm/pkg/contexts/ocm"
)

type handlerFunc func(ocm.ComponentVersionAccess) types.HostFunc

var handlers = make(map[string]handlerFunc)

// GetRegistry returns the registered handlers.
func GetHandlers() map[string]handlerFunc {
	return handlers
}

func register(name string, f handlerFunc) {
	handlers[name] = f
}
