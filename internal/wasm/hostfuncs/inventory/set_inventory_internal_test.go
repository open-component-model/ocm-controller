package inventory

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/stretchr/testify/require"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/cli-utils/pkg/object"
)

func Test_setInventory(t *testing.T) {
	tests := []struct {
		data       string
		want       string
		numEntries int
	}{
		{
			data: `---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: default
data:
  another: test
  third: "3"
`,
		},
		{
			data: `---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: default
data:
  another: test
  third: "3"
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config-2
  namespace: kube-system
data:
`,
		},
	}

	for _, tt := range tests {
		ctx := context.Background()

		pipeline := &v1alpha1.ResourcePipeline{
			Status: v1alpha1.ResourcePipelineStatus{},
		}

		runtime := wazero.NewRuntime(ctx)
		wasi_snapshot_preview1.MustInstantiate(ctx, runtime)
		_, err := runtime.NewHostModuleBuilder("ocm.software").
			NewFunctionBuilder().
			WithFunc(setInventory(pipeline)).
			Export("set_inventory").Instantiate(ctx)
		require.NoError(t, err)

		binary, err := os.ReadFile("./testdata/set_inventory.wasm")
		require.NoError(t, err)

		result := &bytes.Buffer{}
		cfg := wazero.NewModuleConfig().
			WithStdout(result).
			WithArgs(tt.data)

		_, err = runtime.InstantiateWithConfig(ctx, binary, cfg)
		require.NoError(t, err)

		resourceIDs, err := getResourceIDs(strings.NewReader(tt.data))
		require.NoError(t, err)

		for _, entry := range pipeline.GetInventory("test").Entries {
			require.Contains(t, resourceIDs, entry.ID)
		}
	}
}

func getResourceIDs(r io.Reader) ([]string, error) {
	reader := yamlutil.NewYAMLOrJSONDecoder(r, 2048)
	objects := make([]string, 0)

	for {
		obj := &unstructured.Unstructured{}
		err := reader.Decode(obj)
		if err != nil {
			if err == io.EOF {
				err = nil
				break
			}
			return objects, err
		}

		if obj.IsList() {
			err = obj.EachListItem(func(item runtime.Object) error {
				obj := item.(*unstructured.Unstructured)
				objects = append(objects, object.UnstructuredToObjMetadata(obj).String())
				return nil
			})
			if err != nil {
				return objects, err
			}
			continue
		}

		objects = append(objects, object.UnstructuredToObjMetadata(obj).String())
	}

	return objects, nil
}
