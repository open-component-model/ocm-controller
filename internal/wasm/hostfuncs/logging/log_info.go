package logging

import (
	"context"

	"github.com/open-component-model/ocm-controller/internal/wasm/hostfuncs/types"
	wasmerr "github.com/open-component-model/ocm-controller/pkg/wasm/errors"
	"github.com/tetratelabs/wazero/api"
	"golang.org/x/exp/slog"
)

func init() {
	register("log_info", logInfo)
}

func logInfo(logger *slog.Logger) types.HostFunc {
	return func(_ context.Context, m api.Module, offset, size uint32) uint64 {
		mem := m.Memory()

		message, ok := mem.Read(offset, size)
		if !ok {
			return wasmerr.ErrInvalid
		}

		logger.Info(string(message))

		return 0
	}
}
