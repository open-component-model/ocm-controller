// Copyright 2022.
// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// Source defines a possible incoming format for sources that this object needs for further configuration/localization
// steps.
// +kubebuilder:validation:MinProperties=1
type Source struct {
	// +optional
	SourceRef *meta.NamespacedObjectKindReference `json:"sourceRef,omitempty"`
	// +optional
	ResourceRef *ResourceRef `json:"resourceRef,omitempty"`
}

// LocalizationSpec defines the desired state of Localization
type LocalizationSpec struct {
	// +required
	Interval metav1.Duration `json:"interval"`

	// +required
	Source Source `json:"source"`

	// +required
	ConfigRef ConfigReference `json:"configRef"`

	// +required
	SnapshotTemplate SnapshotTemplateSpec `json:"snapshotTemplate"`
}

type ConfigReference struct {
	// +required
	ComponentVersionRef meta.NamespacedObjectReference `json:"componentVersionRef"`

	// +required
	Resource Source `json:"resource"`
}

// ResourceRef define a resource.
// TODO: Change this to ocmmetav1.ResourceReference
// The ocmmetav1.ResourceReference can also contain version!
type ResourceRef struct {
	// +required
	Name string `json:"name"`
	// +optional
	Version string `json:"version,omitempty"`

	// +optional
	ExtraIdentity map[string]string `json:"extraIdentity,omitempty"`

	// ReferencePath is a list of references with identities that include this resource.
	//      referencePath:
	//        - name: installer
	// +optional
	ReferencePath []map[string]string `json:"referencePath,omitempty"`
}

// LocalizationStatus defines the observed state of Localization
type LocalizationStatus struct {
	// ObservedGeneration is the last reconciled generation.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// +optional
	LatestSnapshotDigest string `json:"latestSnapshotDigest,omitempty"`

	LatestConfigVersion string `json:"latestConfigVersion,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Localization is the Schema for the localizations API
type Localization struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LocalizationSpec   `json:"spec,omitempty"`
	Status LocalizationStatus `json:"status,omitempty"`
}

// GetSourceSnapshotKey is a convenient wrapper to get the NamespacedName for a snapshot reference on the object.
func (in Localization) GetSourceSnapshotKey() types.NamespacedName {
	if in.Spec.Source.SourceRef == nil {
		return types.NamespacedName{}
	}
	return types.NamespacedName{
		Namespace: in.Spec.Source.SourceRef.Namespace,
		Name:      in.Spec.Source.SourceRef.Name,
	}
}

// GetRequeueAfter returns the duration after which the Localization must be
// reconciled again.
func (in Localization) GetRequeueAfter() time.Duration {
	return in.Spec.Interval.Duration
}

//+kubebuilder:object:root=true

// LocalizationList contains a list of Localization
type LocalizationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Localization `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Localization{}, &LocalizationList{})
}
