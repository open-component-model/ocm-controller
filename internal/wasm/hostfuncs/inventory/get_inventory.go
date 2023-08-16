package inventory

import (
	"context"
	"encoding/json"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/internal/wasm/hostfuncs/types"
	wasmio "github.com/open-component-model/ocm-controller/internal/wasm/io"
	wasmerr "github.com/open-component-model/ocm-controller/pkg/wasm/errors"
	"github.com/tetratelabs/wazero/api"
)

func init() {
	register("get_inventory", getInventory)
}

// read the inventory objects from the inventory for the current deployer
func getInventory(obj *v1alpha1.ResourcePipeline) types.HostFunc {
	return func(ctx context.Context, m api.Module, offset, size uint32) uint64 {
		mem := m.Memory()

		deployerName, ok := mem.Read(offset, size)
		if !ok {
			return wasmerr.ErrInvalid
		}

		inv := obj.GetInventory(string(deployerName))
		if inv == nil {
			return wasmerr.ErrInvalid
		}

		data, err := json.Marshal(inv)
		if err != nil {
			return wasmerr.ErrEncodingJSON
		}

		return wasmio.Write(ctx, m, offset+size, data)
	}
}
