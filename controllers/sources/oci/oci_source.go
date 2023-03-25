package oci

import (
	"context"
	"fmt"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/source-controller/api/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
)

type Source struct {
	Scheme *runtime.Scheme
	Client client.Client
}

// NewSource creates a new source with given next source provider in the chain.
func NewSource(client client.Client, scheme *runtime.Scheme) *Source {
	return &Source{
		Client: client,
		Scheme: scheme,
	}
}

// CreateSource creates a OCIRepository type source object.
func (s *Source) CreateSource(ctx context.Context, obj *v1alpha1.Snapshot, registryName, name, resourceType string) error {
	logger := log.FromContext(ctx).WithName("oci-source")
	logger.Info("reconciling flux oci source for snapshot for resource type", "type", resourceType)

	ociRepoCR := &v1beta2.OCIRepository{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: obj.GetNamespace(),
			Name:      obj.GetName(),
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, s.Client, ociRepoCR, func() error {
		if ociRepoCR.ObjectMeta.CreationTimestamp.IsZero() {
			if err := controllerutil.SetOwnerReference(obj, ociRepoCR, s.Scheme); err != nil {
				return fmt.Errorf("failed to set owner reference on oci repository source: %w", err)
			}
		}
		ociRepoCR.Spec = v1beta2.OCIRepositorySpec{
			Interval: metav1.Duration{Duration: time.Hour},
			CertSecretRef: &meta.LocalObjectReference{
				Name: "registry-crt",
			},
			URL: fmt.Sprintf("oci://%s/%s", registryName, name),
			Reference: &v1beta2.OCIRepositoryRef{
				Tag: obj.Spec.Tag,
			},
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed o create or update oci repository: %w", err)
	}

	return nil
}
