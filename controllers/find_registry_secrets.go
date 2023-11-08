package controllers

import (
	"context"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type Lister interface {
	client.ObjectList

	List() []client.Object
}

// findRegistrySecrets finds secrets that have a specific key for a given kind.
// Note, this is not working with ObjectList or Unstructured. ObjectList doesn't
// allow listing objects, and unstructured doesn't support fields.
func findRegistrySecrets(c client.Client, key string, list Lister) handler.MapFunc {
	return func(obj client.Object) []reconcile.Request {
		if err := c.List(context.Background(), list, &client.ListOptions{
			FieldSelector: fields.OneTermEqualSelector(key, client.ObjectKeyFromObject(obj).String()),
		}); err != nil {
			return []reconcile.Request{}
		}

		requests := make([]reconcile.Request, len(list.List()))
		for i, item := range list.List() {
			requests[i] = reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      item.GetName(),
					Namespace: item.GetNamespace(),
				},
			}
		}

		return requests
	}
}
