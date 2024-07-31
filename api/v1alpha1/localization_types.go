// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

//nolint:dupl // these are separated for a reason
package v1alpha1

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const LocalizationKind = "Localization"

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=lz
//+kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description=""
//+kubebuilder:printcolumn:name="Source Version",type="string",JSONPath=".status.latestSourceVersion",description=""
//+kubebuilder:printcolumn:name="Config Version",type="string",JSONPath=".status.latestConfigVersion",description=""
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description=""

// Localization is the Schema for the localizations API.
type Localization struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec MutationSpec `json:"spec,omitempty"`
	// +kubebuilder:default={"observedGeneration":-1}
	Status MutationStatus `json:"status,omitempty"`
}

func (in *Localization) GetVID() map[string]string {
	metadata := make(map[string]string)
	metadata[GroupVersion.Group+"/localization_digest"] = in.Status.LatestSnapshotDigest

	return metadata
}

func (in *Localization) SetObservedGeneration(v int64) {
	in.Status.ObservedGeneration = v
}

// GetConditions returns the conditions of the Localization.
func (in *Localization) GetConditions() []metav1.Condition {
	return in.Status.Conditions
}

// SetConditions sets the conditions of the Localization.
func (in *Localization) SetConditions(conditions []metav1.Condition) {
	in.Status.Conditions = conditions
}

// GetRequeueAfter returns the duration after which the Localization must be
// reconciled again.
func (in Localization) GetRequeueAfter() time.Duration {
	return in.Spec.Interval.Duration
}

// GetSnapshotDigest returns the latest snapshot digest for the localization.
func (in Localization) GetSnapshotDigest() string {
	return in.Status.LatestSnapshotDigest
}

// GetSnapshotName returns the key for the snapshot produced by the Localization.
func (in Localization) GetSnapshotName() string {
	return in.Status.SnapshotName
}

// GetSpec returns the mutation spec for a Localization.
func (in *Localization) GetSpec() *MutationSpec {
	return &in.Spec
}

// GetStatus returns the mutation status for a Localization.
func (in *Localization) GetStatus() *MutationStatus {
	return &in.Status
}

// GetObjectMeta returns the object meta for a Localization.
func (in *Localization) GetObjectMeta() *metav1.ObjectMeta {
	return &in.ObjectMeta
}

// GetKind returns the kind for a Localization.
func (in *Localization) GetKind() string {
	return "Localization"
}

func (in *Localization) SetArtifactName(name string) {
	in.Status.ArtifactName = name
}

//+kubebuilder:object:root=true

// LocalizationList contains a list of Localization.
type LocalizationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Localization `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Localization{}, &LocalizationList{})
}
