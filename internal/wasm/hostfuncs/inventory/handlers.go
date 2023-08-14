package inventory

import (
	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/internal/wasm/hostfuncs/types"
	"github.com/tetratelabs/wazero"
)

type handlerFunc func(*v1alpha1.ResourcePipeline) types.HostFunc

var handlers = make(map[string]handlerFunc)

// AddHandlers adds the registered handlers to the builder.
func AddHandlers(b wazero.HostModuleBuilder, obj *v1alpha1.ResourcePipeline) {
	for name, f := range handlers {
		b.NewFunctionBuilder().
			WithFunc(f(obj)).
			Export(name)
	}
}

func register(name string, f handlerFunc) {
	handlers[name] = f
}
