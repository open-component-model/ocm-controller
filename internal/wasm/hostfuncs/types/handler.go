package types

import (
	"context"

	"github.com/tetratelabs/wazero/api"
)

// HostFunc is the signature of a WASM host function.
type HostFunc func(context.Context, api.Module, uint32, uint32) uint64
