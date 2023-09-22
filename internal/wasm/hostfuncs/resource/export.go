package resource

import (
	"github.com/open-component-model/ocm-controller/internal/wasm/hostfuncs/types"
	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	"github.com/tetratelabs/wazero"
)

type handlerFunc func(ocm.ComponentVersionAccess) types.HostFunc

var handlers = make(map[string]handlerFunc)

// Export adds the registered handlers to the builder.
func Export(b wazero.HostModuleBuilder, cv ocm.ComponentVersionAccess) {
	for name, f := range handlers {
		b.NewFunctionBuilder().WithFunc(f(cv)).Export(name)
	}
}

func register(name string, f handlerFunc) {
	handlers[name] = f
}
