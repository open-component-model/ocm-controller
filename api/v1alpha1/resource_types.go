// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"time"

	ocmmetav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ResourceSpec defines the desired state of Resource
type ResourceSpec struct {
	// +required
	Interval metav1.Duration `json:"interval,omitempty"`

	// SourceRef defines the input source from which the resource
	// will be retrieved
	// +required
	SourceRef ObjectReference `json:"sourceRef,omitempty"`

	// Middleware
	Middleware []Middleware `json:"resolvers,omitempty"`

	// Suspend stops all operations on this object.
	// +optional
	Suspend bool `json:"suspend,omitempty"`
}

// Middleware defines a component containing a wasm binary that
// can be used to resolve artifact references
type Middleware struct {
	// +required
	Name string `json:"name"`

	// +required
	Component string `json:"component"`

	// +optional
	// +kubebuilder:default="ghcr.io/phoban01"
	Registry string `json:"registry,omitempty"`

	// +optional
	Values *apiextensionsv1.JSON `json:"values,omitempty"`
}

// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description=""
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message",description=""

// ResourceStatus defines the observed state of Resource
type ResourceStatus struct {
	// ObservedGeneration is the last reconciled generation.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// +optional
	LastAppliedResourceVersion string `json:"lastAppliedResourceVersion,omitempty"`

	// LastAppliedComponentVersion tracks the last applied component version. If there is a change
	// we fire off a reconcile loop to get that new version.
	// +optional
	LastAppliedComponentVersion string `json:"lastAppliedComponentVersion,omitempty"`

	// +optional
	SnapshotName string `json:"snapshotName,omitempty"`

	// +optional
	LatestSnapshotDigest string `json:"latestSnapshotDigest,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=res
//+kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description=""
//+kubebuilder:printcolumn:name="Source Version",type="string",JSONPath=".status.latestSourceVersion",description=""
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description=""

// Resource is the Schema for the resources API
type Resource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ResourceSpec `json:"spec,omitempty"`

	// +kubebuilder:default={"observedGeneration":-1}
	Status ResourceStatus `json:"status,omitempty"`
}

// GetConditions returns the conditions of the Resource.
func (in *Resource) GetConditions() []metav1.Condition {
	return in.Status.Conditions
}

// SetConditions sets the conditions of the Resource.
func (in *Resource) SetConditions(conditions []metav1.Condition) {
	in.Status.Conditions = conditions
}

// GetRequeueAfter returns the duration after which the Resource must be
// reconciled again.
func (in Resource) GetRequeueAfter() time.Duration {
	return in.Spec.Interval.Duration
}

// GetReferencePath returns the component reference path for the resource
func (in Resource) GetReferencePath() []ocmmetav1.Identity {
	return in.Spec.SourceRef.ResourceRef.ReferencePath
}

// GetSnapshotName returns the key for the snapshot produced by the Configuration
func (in Resource) GetSnapshotDigest() string {
	return in.Status.LatestSnapshotDigest
}

// GetSnapshotName returns the key for the snapshot produced by the Configuration
func (in Resource) GetSnapshotName() string {
	return in.Status.SnapshotName
}

//+kubebuilder:object:root=true

// ResourceList contains a list of Resource
type ResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Resource `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Resource{}, &ResourceList{})
}
