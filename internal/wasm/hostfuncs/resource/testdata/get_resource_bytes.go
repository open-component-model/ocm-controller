package main

import (
	"fmt"
	"os"

	"github.com/open-component-model/ocm-controller/pkg/wasm/ocm"
)

func main() {
	resource, err := ocm.GetResourceBytes("data")
	if err != 0 {
		panic(err)
	}
	fmt.Fprintf(os.Stdout, string(resource))
}
