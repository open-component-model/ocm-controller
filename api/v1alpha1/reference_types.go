// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"fmt"
	"strings"

	"github.com/fluxcd/pkg/apis/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ocmmetav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	v3alpha1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/ocm.software/v3alpha1"
)

// ObjectReference defines a resource which may be accessed via a snapshot or component version
// +kubebuilder:validation:MinProperties=1
type ObjectReference struct {
	meta.NamespacedObjectKindReference `json:",inline"`

	// +optional
	ResourceRef *ResourceReference `json:"resourceRef,omitempty"`
}

type ResourceReference struct {
	// +required
	v3alpha1.ElementMeta `json:",inline"`

	// +optional
	ReferencePath []ocmmetav1.Identity `json:"referencePath,omitempty"`
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
