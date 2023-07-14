package resource

import (
	"context"

	"github.com/open-component-model/ocm-controller/internal/wasm/hostfuncs/types"
	wasmio "github.com/open-component-model/ocm-controller/internal/wasm/io"
	wasmerr "github.com/open-component-model/ocm-controller/pkg/wasm/errors"
	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	ocmmetav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	"github.com/tetratelabs/wazero/api"
)

func init() {
	register("get_resource_bytes", getResourceBytes)
}

func getResourceBytes(cv ocm.ComponentVersionAccess) types.HostFunc {
	return func(ctx context.Context, m api.Module, offset, size uint32) uint64 {
		mem := m.Memory()

		// need to do allocation hrer

		name, ok := mem.Read(offset, size)
		if !ok {
			return wasmerr.ErrInvalid
		}

		res, err := cv.GetResource(ocmmetav1.NewIdentity(string(name)))
		if err != nil {
			return wasmerr.ErrResourceNotFound
		}

		meth, err := res.AccessMethod()
		if err != nil {
			return wasmerr.ErrResourceNotAccessible
		}
		defer meth.Close()

		data, err := meth.Get()
		if err != nil {
			return wasmerr.ErrResourceNotAccessible
		}

		return wasmio.Write(ctx, m, offset+size, data)
	}
}
