package inventory

import (
	"bytes"
	"encoding/json"

	"github.com/open-component-model/ocm-controller/pkg/wasm/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/pkg/wasm/convert"
	"github.com/open-component-model/ocm-controller/pkg/wasm/errors"
)

//go:wasmimport ocm.software get_inventory
func _inventoryGet(ptr, size uint32) uint64

//go:wasmimport ocm.software set_inventory
func _inventorySet(ptr, size uint32) uint64

func Get(id string) (*v1alpha1.ResourceInventory, uint64) {
	ptr, size := convert.StringToPointer(id)
	result := _inventoryGet(ptr, size)
	data := bytes.ReplaceAll(convert.PointerToByte(result), []byte("\x00"), []byte{})
	inv := &v1alpha1.ResourceInventory{}
	err := json.Unmarshal(data, inv)
	if err != nil {
		return nil, errors.ErrDecodingJSON
	}
	return inv, 0
}

func Set(object *v1alpha1.SetInventoryRequest) ([]byte, uint64) {
	data, err := json.Marshal(object)
	if err != nil {
		return nil, errors.ErrInvalid
	}
	ptr, size := convert.ByteToPointer(data)
	_inventorySet(ptr, size)
	return nil, 0
}
