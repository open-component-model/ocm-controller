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

func Test_getResourceURL(t *testing.T) {
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
			WithFunc(getResourceURL(cv)).
			Export("get_resource_url").Instantiate(ctx)
		require.NoError(t, err)

		binary, err := os.ReadFile("./testdata/get_resource_url.wasm")
		require.NoError(t, err)

		result := &bytes.Buffer{}
		_, err = runtime.InstantiateWithConfig(ctx, binary, wazero.NewModuleConfig().WithStdout(result))
		require.NoError(t, err)
		require.Equal(t, "ghcr.io/mandelsoft/cnudie/component-descriptors/github.com/vasu1124/introspect@sha256:7f0168496f273c1e2095703a050128114d339c580b0906cd124a93b66ae471e2", result.String())
	}
}
