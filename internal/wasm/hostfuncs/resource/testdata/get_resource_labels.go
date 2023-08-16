package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/open-component-model/ocm-controller/pkg/wasm/ocm"
)

type label struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func main() {
	var labels []label
	labelData, err := ocm.GetResourceLabels("data")
	if err != 0 {
		panic(err)
	}
	if err := json.Unmarshal(labelData, &labels); err != nil {
		panic(err)
	}
	for _, l := range labels {
		fmt.Fprintf(os.Stdout, l.Name+l.Value)
	}
}
