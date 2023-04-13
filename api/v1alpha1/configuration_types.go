// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=cf

// Configuration is the Schema for the configurations API
type Configuration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec MutationSpec `json:"spec,omitempty"`

	// +kubebuilder:default={"observedGeneration":-1}
	Status MutationStatus `json:"status,omitempty"`
}

// GetConditions returns the conditions of the Configuration.
func (in *Configuration) GetConditions() []metav1.Condition {
	return in.Status.Conditions
}

// SetConditions sets the conditions of the Configuration.
func (in *Configuration) SetConditions(conditions []metav1.Condition) {
	in.Status.Conditions = conditions
}

// GetRequeueAfter returns the duration after which the Configuration must be
// reconciled again.
func (in Configuration) GetRequeueAfter() time.Duration {
	return in.Spec.Interval.Duration
}

// GetSnapshotDigest returns the latest snapshot digest for the localization
func (in Configuration) GetSnapshotDigest() string {
	return in.Status.LatestSnapshotDigest
}

// GetSnapshotName returns the key for the snapshot produced by the Localization
func (in Configuration) GetSnapshotName() string {
	return in.Status.SnapshotName
}

// GetSpec returns the mutation spec for a Localization
func (in *Configuration) GetSpec() *MutationSpec {
	return &in.Spec
}

// GetStatus returns the mutation status for a Localization
func (in *Configuration) GetStatus() *MutationStatus {
	return &in.Status
}

//+kubebuilder:object:root=true

// ConfigurationList contains a list of Configuration
type ConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Configuration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Configuration{}, &ConfigurationList{})
}
