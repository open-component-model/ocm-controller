// Copyright 2022.
// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	ocmdesc "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SecretRef is a reference to a secret used to access the OCI repository.
type SecretRef struct {
	Name string `json:"name"`
}

// Repository defines the OCM Repository.
type Repository struct {
	URL       string    `json:"url"`
	SecretRef SecretRef `json:"secretRef"`
}

// Verify holds the secret which contains the signing and verification keys.
type Verify struct {
	SecretRef SecretRef `json:"secretRef"`
}

// ComponentVersionSpec defines the desired state of ComponentVersion
type ComponentVersionSpec struct {
	// +required
	Interval metav1.Duration `json:"interval"`

	// +required
	Name string `json:"name"`

	// +required
	Version string `json:"version"`

	// +required
	Repository Repository `json:"repository"`

	// +required
	Verify Verify `json:"verify"`

	// +optional
	References ReferencesConfig `json:"references,omitempty"`
}

type ReferencesConfig struct {
	// +optional
	Expand bool `json:"expand,omitempty"`
}

// Reference contains all referred components and their versions.
type Reference struct {
	// +required
	Name string `json:"name"`

	// +required
	Version string `json:"version"`

	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	References []Reference `json:"references,omitempty"`

	// +optional
	ExtraIdentity map[string]string `json:"extraIdentity,omitempty"`

	// +optional
	ComponentDescriptorRef meta.NamespacedObjectReference `json:"componentDescriptorRef,omitempty"`
}

// ComponentVersionStatus defines the observed state of ComponentVersion
type ComponentVersionStatus struct {
	ComponentDescriptor Reference `json:"componentDescriptor,omitempty"`

	Verified bool `json:"verified,omitempty"`
}

// GetRequeueAfter returns the duration after which the ComponentVersion must be
// reconciled again.
func (in ComponentVersion) GetRequeueAfter() time.Duration {
	return in.Spec.Interval.Duration
}

// LookupReferenceForIdentity returns the reference that matches up with the given identity selector.
func (in ComponentVersion) LookupReferenceForIdentity(key ocmdesc.IdentitySelector) Reference {
	// Loop through the reference struct in References and return the reference that matches with the
	// given ExtraIdentity.
	return Reference{}
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ComponentVersion is the Schema for the ComponentVersions API
type ComponentVersion struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ComponentVersionSpec   `json:"spec,omitempty"`
	Status ComponentVersionStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ComponentVersionList contains a list of ComponentVersion
type ComponentVersionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComponentVersion `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ComponentVersion{}, &ComponentVersionList{})
}
