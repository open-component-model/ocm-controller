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

// ConfigurationSpec defines the desired state of Configuration
type ConfigurationSpec struct {
	// +required
	Interval metav1.Duration `json:"interval"`

	//TODO@souleb: rename to SnapshotRef
	// +required
	//kubebuilder:default:={snapshot: default}
	SourceRef meta.NamespacedObjectKindReference `json:"sourceRef"`

	// +required
	ConfigRef ConfigReference `json:"configRef"`

	// +required
	SnapshotTemplate SnapshotTemplateSpec `json:"snapshotTemplate"`

	// +optional
	Values map[string]string `json:"values,omitempty"`
}

// ConfigurationStatus defines the observed state of Configuration
type ConfigurationStatus struct {
	// ObservedGeneration is the last reconciled generation.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// +optional
	LatestSnapshotDigest string `json:"latestSnapshotDigest,omitempty"`

	LatestConfigVersion string `json:"latestConfigVersion,omitempty"`
}

func (in Configuration) GetSourceSnapshotKey() types.NamespacedName {
	return types.NamespacedName{
		Namespace: in.Spec.SourceRef.Namespace,
		Name:      in.Spec.SourceRef.Name,
	}
}

// GetRequeueAfter returns the duration after which the Localization must be
// reconciled again.
func (in Configuration) GetRequeueAfter() time.Duration {
	return in.Spec.Interval.Duration
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Configuration is the Schema for the configurations API
type Configuration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConfigurationSpec   `json:"spec,omitempty"`
	Status ConfigurationStatus `json:"status,omitempty"`
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
