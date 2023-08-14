//go:build tinygo.wasm

package inventory

import (
	"github.com/open-component-model/ocm-controller/pkg/wasm/convert"
)

//go:wasmimport ocm.software get_inventory
func _inventoryGet(ptr, size uint32) uint64

//go:wasmimport ocm.software set_inventory
func _inventorySet(ptr, size uint32) uint64

func Get(id string) ([]byte, uint64) {
	ptr, size := convert.StringToPointer(id)
	result := _inventoryGet(ptr, size)
	return []byte(convert.PointerToString(result)), 0
}

func Set(object string) ([]byte, uint64) {
	ptr, size := convert.StringToPointer(object)
	result := _inventorySet(ptr, size)
	return []byte(convert.PointerToString(result)), 0
}
