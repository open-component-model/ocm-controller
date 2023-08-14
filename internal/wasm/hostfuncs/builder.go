package hostfuncs

import (
	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	"github.com/tetratelabs/wazero"
	"golang.org/x/exp/slog"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/internal/wasm/hostfuncs/inventory"
	"github.com/open-component-model/ocm-controller/internal/wasm/hostfuncs/logging"
	"github.com/open-component-model/ocm-controller/internal/wasm/hostfuncs/resource"
)

// ForBuilder adds all registered hostfuncs to the host module builder.
func ForBuilder(b wazero.HostModuleBuilder, obj *v1alpha1.ResourcePipeline, cv ocm.ComponentVersionAccess, logger *slog.Logger) wazero.HostModuleBuilder {
	logging.AddHandlers(b, logger)
	resource.AddHandlers(b, cv)
	inventory.AddHandlers(b, obj)
	return b
}
