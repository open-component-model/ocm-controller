//go:build tinygo.wasm

package convert

// This file is designed to be imported by plugins.

// #include <stdlib.h>
import "C"

import (
	"reflect"
	"unsafe"
)

// PointerToString returns a string from WebAssembly compatible numeric types
// representing its pointer and length.
//
//nolint:govet
func PointerToString(v uint64) string {
	ptr := uint32(v >> 32)
	size := uint32(v & 0xffffffff)
	return string(ptrToByte(ptr, size))
}

// StringToPointer returns a pointer and size pair for the given string in a way
// compatible with WebAssembly numeric types.
// The returned pointer aliases the string hence the string must be kept alive
// until ptr is no longer needed.
func StringToPointer(s string) (uint32, uint32) {
	return byteToPtr([]byte(s))
}

func ptrToByte(ptr, size uint32) []byte {
	var b []byte
	s := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	s.Len = uintptr(size)
	s.Cap = uintptr(size)
	s.Data = uintptr(ptr)
	return b
}

func byteToPtr(buf []byte) (uint32, uint32) {
	if len(buf) == 0 {
		return 0, 0
	}

	size := C.ulong(len(buf))
	ptr := unsafe.Pointer(C.malloc(size))

	copy(unsafe.Slice((*byte)(ptr), size), buf)

	return uint32(uintptr(ptr)), uint32(len(buf))
}

func FreePtr(ptr uint32) {
	C.free(unsafe.Pointer(uintptr(ptr)))
}
