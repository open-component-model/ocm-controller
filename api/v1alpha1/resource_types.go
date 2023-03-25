// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ResourceSpec defines the desired state of Resource
type ResourceSpec struct {
	// +required
	Interval metav1.Duration `json:"interval"`

	// +required
	ComponentVersionRef meta.NamespacedObjectReference `json:"componentVersionRef"`

	// Resource names a Source that this Resource watches.
	// +required
	Resource ResourceRef `json:"resource"`

	// +required
	SnapshotTemplate SnapshotTemplateSpec `json:"snapshotTemplate"`
}

// SnapshotTemplateSpec defines the template used to create snapshots
type SnapshotTemplateSpec struct {
	// +required
	Name string `json:"name"`

	// +optional
	Tag string `json:"tag,omitempty"`

	//TODO@souleb: add a description, is that actually used?
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// +optional
	CreateFluxSource bool `json:"createFluxSource,omitempty"`
}

// ResourceStatus defines the observed state of Resource
type ResourceStatus struct {
	// ObservedGeneration is the last reconciled generation.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +optional
	// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description=""
	// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message",description=""
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// +optional
	LastAppliedResourceVersion string `json:"lastAppliedResourceVersion,omitempty"`

	// LastAppliedComponentVersion tracks the last applied component version. If there is a change
	// we fire off a reconcile loop to get that new version.
	// +optional
	LastAppliedComponentVersion string `json:"lastAppliedComponentVersion,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=cre

// Resource is the Schema for the resources API
type Resource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ResourceSpec   `json:"spec,omitempty"`
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
