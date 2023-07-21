package logging

import (
	"github.com/open-component-model/ocm-controller/pkg/wasm/convert"
	"github.com/open-component-model/ocm-controller/pkg/wasm/errors"
)

//go:wasmimport ocm.software log_info
func _logInfo(ptr, size uint32) uint64

//go:wasmimport ocm.software log_error
func _logError(ptr, size uint32) uint64

// Info logs an info message
func Info(msg string) ([]byte, uint64) {
	ptr, size := convert.StringToPointer(msg)
	result := _logInfo(ptr, size)
	if err := errors.CheckCode([]uint64{result}); err != 0 {
		return nil, err
	}
	return []byte(convert.PointerToString(result)), 0
}

// Error logs an erro message
func Error(msg string) ([]byte, uint64) {
	ptr, size := convert.StringToPointer(msg)
	result := _logError(ptr, size)
	if err := errors.CheckCode([]uint64{result}); err != 0 {
		return nil, err
	}
	return []byte(convert.PointerToString(result)), 0
}
