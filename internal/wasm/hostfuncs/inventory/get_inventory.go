package inventory

import (
	"context"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/internal/wasm/hostfuncs/types"
	wasmio "github.com/open-component-model/ocm-controller/internal/wasm/io"
	wasmerr "github.com/open-component-model/ocm-controller/pkg/wasm/errors"
	"github.com/tetratelabs/wazero/api"
)

func init() {
	register("get_inventory", getInventory)
}

func getInventory(obj *v1alpha1.ResourcePipeline) types.HostFunc {
	return func(ctx context.Context, m api.Module, offset, size uint32) uint64 {
		mem := m.Memory()

		name, ok := mem.Read(offset, size)
		if !ok {
			return wasmerr.ErrInvalid
		}

		return wasmio.Write(ctx, m, name)
	}
}
