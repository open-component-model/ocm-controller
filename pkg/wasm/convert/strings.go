package convert

import "unsafe"

// PointerToString returns a string from WebAssembly compatible numeric types
// representing its pointer and length.
//
//nolint:govet
func PointerToString(v uint64) string {
	ptr := uint32(v >> 32)
	size := uint32(v & 0xffffffff)
	return unsafe.String((*byte)(unsafe.Pointer(uintptr(ptr))), size)
}

// StringToPointer returns a pointer and size pair for the given string in a way
// compatible with WebAssembly numeric types.
// The returned pointer aliases the string hence the string must be kept alive
// until ptr is no longer needed.
func StringToPointer(s string) (uint32, uint32) {
	ptr := unsafe.Pointer(unsafe.StringData(s))
	return uint32(uintptr(ptr)), uint32(len(s))
}
