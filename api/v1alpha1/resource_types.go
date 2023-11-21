// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"time"

	ocmmetav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ResourceSpec defines the desired state of Resource.
type ResourceSpec struct {
	// Interval specifies the interval at which the Repository will be checked for updates.
	// +required
	Interval metav1.Duration `json:"interval"`

	// SourceRef specifies the source object from which the resource should be retrieved.
	// +required
	SourceRef ObjectReference `json:"sourceRef"`

	// Suspend can be used to temporarily pause the reconciliation of the Resource.
	// +optional
	Suspend bool `json:"suspend,omitempty"`
}

// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description=""
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message",description=""

// ResourceStatus defines the observed state of Resource.
type ResourceStatus struct {
	// ObservedGeneration is the last reconciled generation.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions holds the conditions for the ComponentVersion.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// LastAppliedResourceVersion holds the version of the resource that was last applied (if applicable).
	// +optional
	LastAppliedResourceVersion string `json:"lastAppliedResourceVersion,omitempty"`

	// LastAppliedComponentVersion holds the version of the last applied ComponentVersion for the ComponentVersion which contains this Resource.
	// +optional
	LastAppliedComponentVersion string `json:"lastAppliedComponentVersion,omitempty"`

	// SnapshotName specifies the name of the Snapshot that has been created to store the resource
	// within the cluster and make it available for consumption by Flux controllers.
	// +optional
	SnapshotName string `json:"snapshotName,omitempty"`

	// LatestSnapshotDigest is a string representation of the digest for the most recent Resource snapshot.
	// +optional
	LatestSnapshotDigest string `json:"latestSnapshotDigest,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=res
//+kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description=""
//+kubebuilder:printcolumn:name="Source Version",type="string",JSONPath=".status.latestSourceVersion",description=""
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description=""

// Resource is the Schema for the resources API.
type Resource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ResourceSpec `json:"spec,omitempty"`

	// +kubebuilder:default={"observedGeneration":-1}
	Status ResourceStatus `json:"status,omitempty"`
}

func (in *Resource) GetVID() map[string]string {
	metadata := make(map[string]string)
	metadata[GroupVersion.Group+"/resource_version"] = in.Status.LastAppliedResourceVersion

	return metadata
}

func (in *Resource) SetObservedGeneration(v int64) {
	in.Status.ObservedGeneration = v
}

// GetConditions returns the conditions of the Resource.
func (in *Resource) GetConditions() []metav1.Condition {
	return in.Status.Conditions
}

// SetConditions sets the conditions of the Resource.
func (in *Resource) SetConditions(conditions []metav1.Condition) {
	in.Status.Conditions = conditions
}

// GetRequeueAfter returns the duration after which the Resource should be reconciled.
func (in Resource) GetRequeueAfter() time.Duration {
	return in.Spec.Interval.Duration
}

// GetReferencePath returns the component reference path for the Resource.
func (in Resource) GetReferencePath() []ocmmetav1.Identity {
	return in.Spec.SourceRef.ResourceRef.ReferencePath
}

// GetSnapshotDigest returns the digest of the Resource's associated Snapshot.
func (in Resource) GetSnapshotDigest() string {
	return in.Status.LatestSnapshotDigest
}

// GetSnapshotName returns the name of the Resource's associated Snapshot.
func (in Resource) GetSnapshotName() string {
	return in.Status.SnapshotName
}

//+kubebuilder:object:root=true

// ResourceList contains a list of Resource.
type ResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Resource `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Resource{}, &ResourceList{})
}
