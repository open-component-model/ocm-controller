package controllers

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const productDeploymentKind = "ProductDeployment"

// IsProductOwned determines if a given Kubernetes objects has a ProductDeployment owner.
// Used to determine if certain metrics labels need to be updated.
func IsProductOwned(obj client.Object) string {
	for _, ref := range obj.GetOwnerReferences() {
		if ref.Kind == productDeploymentKind {
			return ref.Name
		}
	}

	return ""
}
