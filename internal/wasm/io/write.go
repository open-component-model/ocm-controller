package io

import (
	"context"

	wasmerr "github.com/open-component-model/ocm-controller/pkg/wasm/errors"
	"github.com/tetratelabs/wazero/api"
)

func Write(ctx context.Context, m api.Module, data []byte) uint64 {
	mem := m.Memory()
	malloc := m.ExportedFunction("malloc")
	free := m.ExportedFunction("free")

	size := uint64(len(data))

	results, err := malloc.Call(ctx, size)
	if err != nil {
		return wasmerr.ErrMemoryAllocation
	}

	ptr := results[0]
	defer free.Call(ctx, ptr)

	if !mem.Write(uint32(ptr), data) {
		return wasmerr.ErrWrite
	}

	return ptr<<32 | size
}
