package convert

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
	return string(pointerToByte(ptr, size))
}

//nolint:govet
func PointerToByte(v uint64) []byte {
	ptr := uint32(v >> 32)
	size := uint32(v & 0xffffffff)
	return pointerToByte(ptr, size)
}

// StringToPointer returns a pointer and size pair for the given string in a way
// compatible with WebAssembly numeric types.
// The returned pointer aliases the string hence the string must be kept alive
// until ptr is no longer needed.
func StringToPointer(s string) (uint32, uint32) {
	return ByteToPointer([]byte(s))
}

func pointerToByte(ptr, size uint32) []byte {
	var b []byte
	s := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	s.Len = int(size)
	s.Cap = int(size)
	s.Data = uintptr(ptr)
	return b
}

func ByteToPointer(buf []byte) (uint32, uint32) {
	if len(buf) == 0 {
		return 0, 0
	}

	// Allocate memory using Go's built-in memory management
	data := make([]byte, len(buf))
	copy(data, buf)

	// Get the pointer to the allocated memory
	header := (*reflect.SliceHeader)(unsafe.Pointer(&data))
	ptr := uintptr(header.Data)

	return uint32(ptr), uint32(len(buf))
}
