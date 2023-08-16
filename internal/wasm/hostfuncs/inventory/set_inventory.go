package inventory

import (
	"context"
	"encoding/json"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/internal/wasm/hostfuncs/types"
	wasmapi "github.com/open-component-model/ocm-controller/pkg/wasm/api/v1alpha1"
	wasmerr "github.com/open-component-model/ocm-controller/pkg/wasm/errors"
	"github.com/tetratelabs/wazero/api"
)

func init() {
	register("set_inventory", setInventory)
}

// update the inventory objects from the inventory for the current deployer
func setInventory(obj *v1alpha1.ResourcePipeline) types.HostFunc {
	return func(ctx context.Context, m api.Module, offset, size uint32) uint64 {
		mem := m.Memory()

		data, ok := mem.Read(offset, size)
		if !ok {
			return wasmerr.ErrInvalid
		}

		var req wasmapi.SetInventoryRequest
		err := json.Unmarshal(data, &req)
		if err != nil {
			return wasmerr.ErrDecodingJSON
		}

		obj.SetInventory(req.Step, req.Inventory)

		return 0
	}
}
