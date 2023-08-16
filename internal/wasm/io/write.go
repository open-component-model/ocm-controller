package io

import (
	"context"

	"github.com/tetratelabs/wazero/api"
)

func Write(ctx context.Context, m api.Module, offset uint32, data []byte) uint64 {
	mem := m.Memory()

	size := uint64(len(data))

	if !mem.Write(offset, data) {
		panic("write error")
	}

	return uint64(offset)<<32 | size
}
