// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	ocmmetav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SnapshotWriter defines any object which produces a snapshot
// +k8s:deepcopy-gen=false
type SnapshotWriter interface {
	client.Object
	GetSnapshotDigest() string
	GetSnapshotName() string
}

// SnapshotSpec defines the desired state of Snapshot.
type SnapshotSpec struct {
	Identity ocmmetav1.Identity `json:"identity"`

	Digest string `json:"digest"`

	Tag string `json:"tag"`

	// Suspend stops all operations on this object.
	// +optional
	Suspend bool `json:"suspend,omitempty"`
}

// SnapshotStatus defines the observed state of Snapshot.
type SnapshotStatus struct {
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Digest is calculated by the caching layer.
	// +optional
	LastReconciledDigest string `json:"digest,omitempty"`

	// Tag defines the explicit tag that was used to create the related snapshot and cache entry.
	// +optional
	LastReconciledTag string `json:"tag,omitempty"`

	// RepositoryURL has the concrete URL pointing to the local registry including the service name.
	// +optional
	RepositoryURL string `json:"repositoryURL,omitempty"`

	// ObservedGeneration is the last reconciled generation.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

func (in *Snapshot) GetVID() map[string]string {
	metadata := make(map[string]string)
	metadata[GroupVersion.Group+"/snapshot_digest"] = in.Status.LastReconciledDigest

	return metadata
}

func (in *Snapshot) SetObservedGeneration(v int64) {
	in.Status.ObservedGeneration = v
}

// GetComponentVersion returns the component version for the snapshot.
func (in Snapshot) GetComponentVersion() string {
	return in.Spec.Identity[ComponentVersionKey]
}

// GetComponentResourceVersion returns the resource version for the snapshot.
func (in Snapshot) GetComponentResourceVersion() string {
	return in.Spec.Identity[ResourceVersionKey]
}

// GetDigest returns the last reconciled digest for the snapshot.
func (in Snapshot) GetDigest() string {
	return in.Status.LastReconciledDigest
}

// GetConditions returns the status conditions of the object.
func (in Snapshot) GetConditions() []metav1.Condition {
	return in.Status.Conditions
}

// SetConditions sets the status conditions on the object.
func (in *Snapshot) SetConditions(conditions []metav1.Condition) {
	in.Status.Conditions = conditions
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=snap
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description=""
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message",description=""

// Snapshot is the Schema for the snapshots API.
type Snapshot struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SnapshotSpec   `json:"spec,omitempty"`
	Status SnapshotStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SnapshotList contains a list of Snapshot.
type SnapshotList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Snapshot `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Snapshot{}, &SnapshotList{})
}
