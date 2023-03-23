package helm

import (
	"context"
	"fmt"
	"time"

	"github.com/fluxcd/source-controller/api/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/controllers/sources"
)

const supportedType = "helmChart"

type Source struct {
	Scheme *runtime.Scheme
	Client client.Client
	Next   sources.FluxSource
}

// NewSource creates a new source with given next source provider in the chain.
func NewSource(client client.Client, scheme *runtime.Scheme, next sources.FluxSource) *Source {
	return &Source{
		Scheme: scheme,
		Client: client,
		Next:   next,
	}
}

// CreateSource creates a HelmRepository type source object.
func (s *Source) CreateSource(ctx context.Context, obj *v1alpha1.Snapshot, registryName, name, resourceType string) error {
	if resourceType != supportedType {
		if s.Next == nil {
			return fmt.Errorf("no next source creator defined for resource type '%s'", resourceType)
		}

		return s.Next.CreateSource(ctx, obj, registryName, name, resourceType)
	}

	logger := log.FromContext(ctx).WithName("helm-source")
	logger.Info("reconciling flux helm source for snapshot")

	helmRepoCR := &v1beta2.HelmRepository{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: obj.GetNamespace(),
			Name:      obj.GetName(),
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, s.Client, helmRepoCR, func() error {
		if helmRepoCR.ObjectMeta.CreationTimestamp.IsZero() {
			if err := controllerutil.SetOwnerReference(obj, helmRepoCR, s.Scheme); err != nil {
				return fmt.Errorf("failed to set owner reference on oci repository source: %w", err)
			}
		}
		helmRepoCR.Spec = v1beta2.HelmRepositorySpec{
			Interval: metav1.Duration{Duration: time.Hour},
			URL:      fmt.Sprintf("oci://%s/%s:%s", registryName, name, obj.Spec.Tag),
			Type:     "oci",
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed o create or helm repository: %w", err)
	}

	return nil
}
