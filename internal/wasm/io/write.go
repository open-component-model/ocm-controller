package io

import (
	"context"
	"math"

	wasmerr "github.com/open-component-model/ocm-controller/pkg/wasm/errors"
	"github.com/tetratelabs/wazero/api"
)

// Write creates a pointer to data using data size.
func Write(ctx context.Context, m api.Module, data []byte) (result uint64) {
	mem := m.Memory()
	malloc := m.ExportedFunction("malloc")
	free := m.ExportedFunction("free")

	size := uint64(len(data))

	results, err := malloc.Call(ctx, size)
	if err != nil {
		return wasmerr.ErrMemoryAllocation
	}

	ptr := results[0]
	defer func() {
		if _, err := free.Call(ctx, ptr); err != nil {
			result = wasmerr.ErrMemoryAllocation
		}
	}()

	if ptr > math.MaxUint32 {
		return wasmerr.ErrWrite
	}
	if !mem.Write(uint32(ptr), data) {
		return wasmerr.ErrWrite
	}

	return ptr<<32 | size
}
