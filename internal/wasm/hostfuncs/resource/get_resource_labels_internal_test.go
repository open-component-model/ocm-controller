package resource

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

func Test_getResourceLabels(t *testing.T) {
	tests := []struct {
		data []byte
	}{
		{data: []byte("testdata")},
	}

	for _, tt := range tests {
		ctx := context.Background()
		cv, err := NewTestComponentWithData(t, tt.data)
		require.NoError(t, err)

		runtime := wazero.NewRuntime(ctx)
		wasi_snapshot_preview1.MustInstantiate(ctx, runtime)
		_, err = runtime.NewHostModuleBuilder("ocm.software").
			NewFunctionBuilder().
			WithFunc(getResourceLabels(cv)).
			Export("get_resource_labels").Instantiate(ctx)
		require.NoError(t, err)

		binary, err := os.ReadFile("./testdata/get_resource_labels.wasm")
		require.NoError(t, err)

		result := &bytes.Buffer{}
		mod, err := runtime.InstantiateWithConfig(ctx, binary, wazero.NewModuleConfig().WithStdout(result))
		require.NoError(t, err)

		handler := mod.ExportedFunction("handler")
		_, err = handler.Call(ctx)
		require.NoError(t, err)
		require.Equal(t, string(tt.data), result.String())
	}
}
