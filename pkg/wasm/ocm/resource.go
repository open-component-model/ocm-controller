//go:build tinygo.wasm

package ocm

import (
	"github.com/open-component-model/ocm-controller/pkg/wasm/convert"
	"github.com/open-component-model/ocm-controller/pkg/wasm/errors"
)

//go:wasmimport ocm.software get_resource_bytes
func _getResourceBytes(ptr, size uint32) uint64

//go:wasmimport ocm.software get_resource_labels
func _getResourceLabels(ptr, size uint32) uint64

//go:wasmimport ocm.software get_resource_url
func _getResourceURL(ptr, size uint32) uint64

// GetResourceBytes takes a resource name and returns the access location for the resource
func GetResourceBytes(resource string) ([]byte, uint64) {
	ptr, size := convert.StringToPointer(resource)
	result := _getResourceBytes(ptr, size)
	if err := errors.CheckCode([]uint64{result}); err != 0 {
		return nil, err
	}
	return []byte(convert.PointerToString(result)), 0
}

// GetResourceLabels takes a resource name and returns the access location for the resource
func GetResourceLabels(resource string) ([]byte, uint64) {
	ptr, size := convert.StringToPointer(resource)
	result := _getResourceLabels(ptr, size)
	if err := errors.CheckCode([]uint64{result}); err != 0 {
		return nil, err
	}
	return []byte(convert.PointerToString(result)), 0
}

// GetResourceURL takes a resource name and returns the access location for the resource
func GetResourceURL(resource string) (string, uint64) {
	ptr, size := convert.StringToPointer(resource)
	result := _getResourceURL(ptr, size)
	if err := errors.CheckCode([]uint64{result}); err != 0 {
		return "", err
	}
	return convert.PointerToString(result), 0
}
