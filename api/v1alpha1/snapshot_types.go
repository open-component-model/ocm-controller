// Copyright 2022.
// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"fmt"

	"github.com/mitchellh/hashstructure/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ComponentNameKey          = "component-name"
	ComponentVersionKey       = "component-version"
	ResourceNameKey           = "resource-name"
	ResourceVersionKey        = "resource-version"
	SourceNameKey             = "source-name"
	SourceNamespaceKey        = "source-namespace"
	SourceArtifactChecksumKey = "source-artifact-checksum"
)

// SnapshotSpec defines the desired state of Snapshot
type SnapshotSpec struct {
	Identity Identity `json:"identity"`

	// +optional
	CreateFluxSource bool `json:"createFluxSource,omitempty"`

	Digest string `json:"digest"`

	Tag string `json:"tag"`
}

// SnapshotStatus defines the observed state of Snapshot
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
}

// Identity defines a cache entry. It is used to generate a hash that is then used by the
// caching layer to identify an entry.
// +kubebuilder:validation:MaxProperties=20
type Identity map[string]string

// Hash calculates the hash of an identity
func (i *Identity) Hash() (string, error) {
	hash, err := hashstructure.Hash(i, hashstructure.FormatV2, nil)
	if err != nil {
		return "", fmt.Errorf("failed to hash identity: %w", err)
	}

	return fmt.Sprintf("sha-%d", hash), nil
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

// GetConditions returns the status conditions of the object.
func (in Snapshot) GetConditions() []metav1.Condition {
	return in.Status.Conditions
}

// SetConditions sets the status conditions on the object.
func (in *Snapshot) SetConditions(conditions []metav1.Condition) {
	in.Status.Conditions = conditions
}

// GetStatusConditions returns a pointer to the Status.Conditions slice.
// Deprecated: use GetConditions instead.
func (in *Snapshot) GetStatusConditions() *[]metav1.Condition {
	return &in.Status.Conditions
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
