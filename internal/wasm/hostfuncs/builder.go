package hostfuncs

import (
	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	"github.com/tetratelabs/wazero"
	"golang.org/x/exp/slog"

	"github.com/open-component-model/ocm-controller/internal/wasm/hostfuncs/logging"
	"github.com/open-component-model/ocm-controller/internal/wasm/hostfuncs/resource"
)

// ForBuilder adds all registered hostfuncs to the host module builder.
func ForBuilder(b wazero.HostModuleBuilder, cv ocm.ComponentVersionAccess, logger *slog.Logger) wazero.HostModuleBuilder {
	// register the logging function handlers
	for name, f := range logging.GetHandlers() {
		b = b.NewFunctionBuilder().
			WithFunc(f(logger)).
			Export(name)
	}

	// register the resource function handlers
	for name, f := range resource.GetHandlers() {
		b = b.NewFunctionBuilder().
			WithFunc(f(cv)).
			Export(name)
	}
	return b
}
