package inventory

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	wasmapi "github.com/open-component-model/ocm-controller/pkg/wasm/api/v1alpha1"
	"github.com/stretchr/testify/require"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

func Test_getInventory(t *testing.T) {
	tests := []struct {
		data []string
	}{
		{
			data: []string{
				"default_test-config__ConfigMap",
			},
		},
		{
			data: []string{
				"default_test-config__ConfigMap",
				"kube-system_test-config-2__ConfigMap",
				"ocm-system_test-config-3__Deployment",
			},
		},
	}

	for _, tt := range tests {
		ctx := context.Background()

		entries := make([]wasmapi.ResourceRef, 0)
		for _, v := range tt.data {
			entries = append(entries, wasmapi.ResourceRef{
				ID:      v,
				Version: "v1",
			})
		}

		pipeline := &v1alpha1.ResourcePipeline{
			Status: v1alpha1.ResourcePipelineStatus{
				DeployerInventories: map[string]*wasmapi.ResourceInventory{
					"test": &wasmapi.ResourceInventory{
						Entries: entries,
					},
				},
			},
		}

		runtime := wazero.NewRuntime(ctx)
		wasi_snapshot_preview1.MustInstantiate(ctx, runtime)
		_, err := runtime.NewHostModuleBuilder("ocm.software").
			NewFunctionBuilder().
			WithFunc(getInventory(pipeline)).
			Export("get_inventory").Instantiate(ctx)
		require.NoError(t, err)

		binary, err := os.ReadFile("./testdata/get_inventory.wasm")
		require.NoError(t, err)

		result := &bytes.Buffer{}
		cfg := wazero.NewModuleConfig().
			WithStdout(result).
			WithArgs("test")

		_, err = runtime.InstantiateWithConfig(ctx, binary, cfg)
		require.NoError(t, err)
		require.Equal(t, tt.data, strings.Split(result.String(), "\n"))
	}
}
