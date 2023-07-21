package resource

import (
	"context"
	"encoding/json"

	"github.com/open-component-model/ocm-controller/internal/wasm/hostfuncs/types"
	wasmio "github.com/open-component-model/ocm-controller/internal/wasm/io"
	wasmerr "github.com/open-component-model/ocm-controller/pkg/wasm/errors"
	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	ocmmetav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	"github.com/tetratelabs/wazero/api"
)

func init() {
	register("get_resource_labels", getResourceLabels)
}

func getResourceLabels(cv ocm.ComponentVersionAccess) types.HostFunc {
	return func(ctx context.Context, m api.Module, offset, size uint32) uint64 {
		mem := m.Memory()

		name, ok := mem.Read(offset, size)
		if !ok {
			return wasmerr.ErrInvalid
		}

		res, err := cv.GetResource(ocmmetav1.NewIdentity(string(name)))
		if err != nil {
			return wasmerr.ErrResourceNotAccessible
		}

		data, err := json.Marshal(res.Meta().Labels)
		if err != nil {
			return wasmerr.ErrEncodingJSON
		}

		return wasmio.Write(ctx, m, data)
	}
}
