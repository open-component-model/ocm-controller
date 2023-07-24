//go:build tinygo.wasm

package main

import (
	"fmt"
	"os"

	"github.com/open-component-model/ocm-controller/pkg/wasm/ocm"
)

// build using:
// tinygo build -o ./get_resource_bytes.wasm -panic=trap -scheduler=none -target=wasi ./get_resource_bytes.go

func main() {}

//export handler
func handler() uint64 {
	resource, err := ocm.GetResourceBytes("data")
	if err != 0 {
		return err
	}
	fmt.Fprintf(os.Stdout, string(resource))
	return 0
}
