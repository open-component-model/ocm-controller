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

type module struct {
	name   string
	wasm   []byte
	object *v1alpha1.ResourcePipeline
	cv     ocm.ComponentVersionAccess
	dir    string
	logger *slog.Logger
}

func New(name string, wasm []byte) *module {
	return &module{
		name: name,
		wasm: wasm,
	}
}

func (m *module) WithLogger(l *slog.Logger) *module {
	m.logger = l
	return m
}

func (m *module) WithComponent(cv ocm.ComponentVersionAccess) *module {
	m.cv = cv
	return m
}

func (m *module) WithObject(obj *v1alpha1.ResourcePipeline) *module {
	m.object = obj
	return m
}

func (m *module) WithDir(path string) *module {
	m.dir = path
	return m
}

func (m *module) Run(ctx context.Context, args ...string) error {
	runtime := wazero.NewRuntime(ctx)
	defer runtime.Close(ctx)

	if err := hostfuncs.Export(ctx, runtime, m.object, m.cv, m.logger); err != nil {
		return err
	}
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, runtime); err != nil {
		return err
	}

	cfg := wazero.NewModuleConfig().
		WithStdout(os.Stdout).
		WithArgs(m.name, args[0]).
		WithFSConfig(wazero.NewFSConfig().WithDirMount(m.dir, "/data"))

	if m.dir != "" {
		cfg = cfg.WithEnv("OCM_SOFTWARE_DATA_DIR", "/data")
	}

	mod, err := runtime.InstantiateWithConfig(ctx, m.wasm, cfg)
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
