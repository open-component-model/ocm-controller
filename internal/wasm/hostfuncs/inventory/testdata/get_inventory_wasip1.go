package main

import (
	"fmt"
	"os"

	"github.com/open-component-model/ocm-controller/pkg/wasm/inventory"
)

func main() {
	inv, _ := inventory.Get(os.Args[0])
	for i, entry := range inv.Entries {
		str := entry.ID
		if i != len(inv.Entries)-1 {
			str += "\n"
		}
		fmt.Fprintf(os.Stdout, str)
	}
}
