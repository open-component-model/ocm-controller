// Copyright 2022.
// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SnapshotSpec defines the desired state of Snapshot
type SnapshotSpec struct {
	Ref    string `json:"ref"`
	Digest string `json:"digest"`
}

// SnapshotStatus defines the observed state of Snapshot
type SnapshotStatus struct {
	// +optional
	Image string `json:"image,omitempty"`

	// +optional
	Layer string `json:"layer,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Snapshot is the Schema for the snapshots API
type Snapshot struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SnapshotSpec   `json:"spec,omitempty"`
	Status SnapshotStatus `json:"status,omitempty"`
}

func (in Snapshot) GetDigest() string {
	if in.Status.Layer == "" || !strings.Contains(in.Status.Layer, "@") {
		return ""
	}

	return strings.Split(in.Status.Layer, "@")[1]
}

func (in Snapshot) GetBlob() string {
	if in.Status.Layer == "" || !strings.Contains(in.Status.Layer, "@") {
		return ""
	}

	return strings.TrimPrefix(in.Status.Layer, "http://")
}

//+kubebuilder:object:root=true

// SnapshotList contains a list of Snapshot
type SnapshotList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Snapshot `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Snapshot{}, &SnapshotList{})
}
