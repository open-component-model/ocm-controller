package sources

import (
	"context"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
)

// FluxSource defines the capability of creating sources for flux to pick up.
type FluxSource interface {
	// CreateSource generates a source object and adds `obj` as an owner.
	CreateSource(ctx context.Context, obj *v1alpha1.Snapshot, registryName, name, resourceType string) error
}
