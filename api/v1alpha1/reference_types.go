package v1alpha1

import (
	"fmt"
	"strings"

	"github.com/fluxcd/pkg/apis/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ocmmetav1 "ocm.software/ocm/api/ocm/compdesc/meta/v1"
)

// ObjectReference defines a resource which may be accessed via a snapshot or component version
// +kubebuilder:validation:MinProperties=1
type ObjectReference struct {
	meta.NamespacedObjectKindReference `json:",inline"`

	// ResourceRef defines what resource to fetch.
	// +optional
	ResourceRef *ResourceReference `json:"resourceRef,omitempty"`
}

type ResourceReference struct {
	ElementMeta `json:",inline"`

	// +optional
	ReferencePath []ocmmetav1.Identity `json:"referencePath,omitempty"`
}

type ElementMeta struct {
	// +required
	Name string `json:"name"`

	// +optional
	Version string `json:"version,omitempty"`

	// +optional
	ExtraIdentity ocmmetav1.Identity `json:"extraIdentity,omitempty"`

	// +optional
	Labels ocmmetav1.Labels `json:"labels,omitempty"`
}

func (o *ObjectReference) GetNamespacedName() string {
	return fmt.Sprintf("%s/%s", o.Namespace, o.Name)
}

func (o *ObjectReference) GetObjectKey() client.ObjectKey {
	return client.ObjectKey{Namespace: o.Namespace, Name: o.Name}
}

func (o *ObjectReference) GetGVR() schema.GroupVersionResource {
	gvk := schema.FromAPIVersionAndKind(o.APIVersion, o.Kind)
	// Replace the kind with the resource name
	resource := strings.ToLower(gvk.Kind) + "s"

	return schema.GroupVersionResource{Group: gvk.Group, Version: gvk.Version, Resource: resource}
}

func (o *ObjectReference) GetVersion() string {
	if o.ResourceRef == nil {
		return ""
	}

	return o.ResourceRef.Version
}
