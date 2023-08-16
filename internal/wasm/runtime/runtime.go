package runtime

import (
	"context"
	"os"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/internal/wasm/hostfuncs"
	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"golang.org/x/exp/slog"
)

var (
	moduleName = "ocm.software"
)

type Module struct {
	logger     *slog.Logger
	object     *v1alpha1.ResourcePipeline
	cv         ocm.ComponentVersionAccess
	resource   string
	dir        string
	finalizers []func() error
}

func NewModule(resource string, logger *slog.Logger, obj *v1alpha1.ResourcePipeline, cv ocm.ComponentVersionAccess, dir string) *Module {
	return &Module{
		resource:   resource,
		logger:     logger,
		object:     obj,
		cv:         cv,
		dir:        dir,
		finalizers: make([]func() error, 0),
	}
}

func (m *Module) Run(ctx context.Context, config, binary []byte) error {
	runtimeConfig := wazero.NewRuntimeConfig().WithCloseOnContextDone(true)
	runtime := wazero.NewRuntimeWithConfig(ctx, runtimeConfig)
	m.AddFinalizer(func() error { return runtime.Close(ctx) })

	wasi_snapshot_preview1.MustInstantiate(ctx, runtime)

	fsConfig := wazero.NewFSConfig().WithDirMount(m.dir, "/data")

	modConfig := wazero.NewModuleConfig().
		WithArgs(m.resource, string(config)).
		WithStdout(os.Stdout).
		WithFSConfig(fsConfig)

	builder := runtime.NewHostModuleBuilder(moduleName)
	builder = hostfuncs.ForBuilder(builder, m.object, m.cv, m.logger)
	if _, err := builder.Instantiate(ctx); err != nil {
		return err
	}

	_, err = runtime.InstantiateWithConfig(ctx, binary, modConfig)
	if err != nil {
		return err
	}

	// handler := mod.ExportedFunction("handler")
	// result, err := handler.Call(ctx)
	// if err != nil {
	//     return err
	// }
	// if err := wasmerr.Check(result); err != nil {
	//     return err
	// }

	return nil
}

func (m *Module) AddFinalizer(f func() error) {
	m.finalizers = append(m.finalizers, f)
}

func (m *Module) Close() error {
	for _, f := range m.finalizers {
		if err := f(); err != nil {
			return err
		}
	}
	return nil
}
