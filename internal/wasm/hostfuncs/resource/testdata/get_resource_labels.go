//go:build tinygo.wasm

package main

import (
	"encoding/json"
	"fmt"
	"os"

	wasmerr "github.com/open-component-model/ocm-controller/pkg/wasm/errors"
	"github.com/open-component-model/ocm-controller/pkg/wasm/ocm"
)

// build using:
// tinygo build -o ./get_resource_labels.wasm -panic=trap -scheduler=none -target=wasi ./get_resource_labels.go

func main() {}

type label struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

//export handler
func handler() uint64 {
	var labels []label
	labelData, err := ocm.GetResourceLabels("data")
	if err != 0 {
		return err
	}
	if err := json.Unmarshal(labelData, &labels); err != nil {
		return wasmerr.ErrDecodingJSON
	}
	for _, l := range labels {
		fmt.Fprintf(os.Stdout, l.Name+l.Value)
	}
	return 0
}
