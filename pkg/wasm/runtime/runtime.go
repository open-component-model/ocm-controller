package runtime

import (
	"context"
	"os"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"golang.org/x/exp/slog"

	"github.com/open-component-model/ocm-controller/internal/wasm/hostfuncs"
	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

type Runtime struct {
	runtime wazero.Runtime
	object  *v1alpha1.ResourcePipeline
	cv      ocm.ComponentVersionAccess
	dir     string
	logger  *slog.Logger
}

func New() *Runtime {
	return &Runtime{}
}

func (m *Runtime) WithLogger(l *slog.Logger) *Runtime {
	m.logger = l

	return m
}

func (m *Runtime) WithComponent(cv ocm.ComponentVersionAccess) *Runtime {
	m.cv = cv

	return m
}

func (m *Runtime) WithObject(obj *v1alpha1.ResourcePipeline) *Runtime {
	m.object = obj

	return m
}

func (m *Runtime) WithDir(path string) *Runtime {
	m.dir = path

	return m
}

func (m *Runtime) Close(ctx context.Context) error {
	return m.runtime.Close(ctx)
}

func (m *Runtime) Init(ctx context.Context) error {
	r := wazero.NewRuntimeWithConfig(ctx,
		wazero.NewRuntimeConfig().WithCloseOnContextDone(true))

	if err := hostfuncs.Export(ctx, r, m.cv, m.logger); err != nil {
		return err
	}
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, r); err != nil {
		return err
	}
	m.runtime = r

	return nil
}

func (m *Runtime) Call(ctx context.Context, name string, wasm []byte, args ...string) error {
	cfg := wazero.NewModuleConfig().
		WithStdout(os.Stdout).
		WithArgs(name, args[0]).
		WithFSConfig(wazero.NewFSConfig().WithDirMount(m.dir, "/data"))

	if m.dir != "" {
		cfg = cfg.WithEnv("OCM_SOFTWARE_DATA_DIR", "/data")
	}

	mod, err := m.runtime.InstantiateWithConfig(ctx, wasm, cfg)
	if err != nil {
		return err
	}

	handler := mod.ExportedFunction("run")
	_, err = handler.Call(ctx)
	if err != nil {
		return err
	}

	return nil
}
