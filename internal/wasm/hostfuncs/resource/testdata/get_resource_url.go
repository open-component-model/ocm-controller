//go:build tinygo.wasm

package main

import (
	"fmt"
	"os"

	"github.com/open-component-model/ocm-controller/pkg/wasm/ocm"
)

// build using:
// tinygo build -o ./get_resource_url.wasm -panic=trap -scheduler=none -target=wasi ./get_resource_url.go

func main() {}

//export handler
func getURLHandler() uint64 {
	url, err := ocm.GetResourceURL("data")
	if err != 0 {
		return err
	}
	fmt.Fprintf(os.Stdout, string(url))
	return 0
}
