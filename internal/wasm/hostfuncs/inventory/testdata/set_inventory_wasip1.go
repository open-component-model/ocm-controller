package main

import (
	"io"
	"os"
	"strings"

	"github.com/open-component-model/ocm-controller/pkg/wasm/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/pkg/wasm/inventory"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
)

func main() {
	objects, err := readObjects(strings.NewReader(os.Args[0]))
	if err != nil {
		panic(err)
	}

	inv := &v1alpha1.ResourceInventory{}
	cs := v1alpha1.NewChangeSet()

	for _, object := range objects {
		cs.AddEntry(object)
	}

	inv.AddChangeSet(cs)

	request := &v1alpha1.SetInventoryRequest{
		Step:      "test",
		Inventory: inv,
	}

	inventory.Set(request)
}

func readObjects(r io.Reader) ([]*unstructured.Unstructured, error) {
	reader := yamlutil.NewYAMLOrJSONDecoder(r, 2048)
	objects := make([]*unstructured.Unstructured, 0)

	for {
		obj := &unstructured.Unstructured{}
		err := reader.Decode(obj)
		if err != nil {
			if err == io.EOF {
				err = nil
				break
			}
			return objects, err
		}

		if obj.IsList() {
			err = obj.EachListItem(func(item runtime.Object) error {
				obj := item.(*unstructured.Unstructured)
				objects = append(objects, obj)
				return nil
			})
			if err != nil {
				return objects, err
			}
			continue
		}

		objects = append(objects, obj)
	}

	return objects, nil
}
