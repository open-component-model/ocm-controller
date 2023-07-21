package resource

import (
	"context"
	"errors"
	"fmt"

	"github.com/open-component-model/ocm-controller/internal/wasm/hostfuncs/types"
	wasmio "github.com/open-component-model/ocm-controller/internal/wasm/io"
	wasmerr "github.com/open-component-model/ocm-controller/pkg/wasm/errors"
	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/accessmethods/localblob"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/accessmethods/ociartifact"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/accessmethods/ociblob"
	ocmmetav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	"github.com/tetratelabs/wazero/api"
)

func init() {
	register("get_resource_url", getResourceURL)
}

func getResourceURL(cv ocm.ComponentVersionAccess) types.HostFunc {
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

		ref, err := getReference(cv.GetContext(), res)
		if err != nil {
			return wasmerr.ErrResourceNotAccessible
		}

		return wasmio.Write(ctx, m, []byte(ref))
	}
}

func getReference(octx ocm.Context, res ocm.ResourceAccess) (string, error) {
	accSpec, err := res.Access()
	if err != nil {
		return "", err
	}

	var (
		ref    string
		refErr error
	)

	for ref == "" && refErr == nil {
		switch x := accSpec.(type) {
		case *ociartifact.AccessSpec:
			ref = x.ImageReference
		case *ociblob.AccessSpec:
			ref = fmt.Sprintf("%s@%s", x.Reference, x.Digest)
		case *localblob.AccessSpec:
			if x.GlobalAccess == nil {
				refErr = errors.New("cannot determine image digest")
			} else {
				accSpec, refErr = octx.AccessSpecForSpec(x.GlobalAccess)
			}
		default:
			refErr = errors.New("cannot determine access spec type")
		}
	}

	return ref, nil
}
