package hostfuncs

import (
	"context"

	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	"github.com/tetratelabs/wazero"
	"golang.org/x/exp/slog"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/internal/wasm/hostfuncs/logging"
	"github.com/open-component-model/ocm-controller/internal/wasm/hostfuncs/resource"
)

const module = "ocm.software"

// Export adds hostfuncs to the host module builder.
func Export(
	ctx context.Context,
	runtime wazero.Runtime,
	obj *v1alpha1.ResourcePipeline,
	cv ocm.ComponentVersionAccess,
	logger *slog.Logger,
) error {
	b := runtime.NewHostModuleBuilder(module)

	// add the logging functions
	logging.Export(b, logger)

	// add the ocm resources functions
	resource.Export(b, cv)

	if _, err := b.Instantiate(ctx); err != nil {
		return err
	}

	return nil
}
