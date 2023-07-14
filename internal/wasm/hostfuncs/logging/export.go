package logging

import (
	"github.com/open-component-model/ocm-controller/internal/wasm/hostfuncs/types"
	"github.com/tetratelabs/wazero"
	"golang.org/x/exp/slog"
)

type handlerFunc func(*slog.Logger) types.HostFunc

var handlers = make(map[string]handlerFunc)

// Export adds the registered handlers to the builder.
func Export(b wazero.HostModuleBuilder, logger *slog.Logger) {
	for name, f := range handlers {
		b.NewFunctionBuilder().WithFunc(f(logger)).Export(name)
	}
}

func register(name string, f handlerFunc) {
	handlers[name] = f
}
