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
	ContentTypeKey            = "delivery.ocm.software/content-type"
)

// SnapshotSpec defines the desired state of Snapshot
type SnapshotSpec struct {
	Identity Identity `json:"identity"`

	// +optional
	CreateFluxSource bool `json:"createFluxSource,omitempty"`

	Digest string `json:"digest"`

	Tag string `json:"tag"`

	// DuplicateTagToTag defines a tag to which the current tag can be duplicated.
	// Useful to define a fallback tag or a specific version in the OCI cache.
	// +optional
	DuplicateTagToTag string `json:"duplicateTagToTag,omitempty"`
}

// SnapshotStatus defines the observed state of Snapshot
type SnapshotStatus struct {
	// +optional
	// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description=""
	// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message",description=""
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
//+kubebuilder:resource:shortName=cs

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

// GetContentType returns the snapshot's content type.
func (in Snapshot) GetContentType() string {
	return in.Annotations[ContentTypeKey]
}

// SetContentType is just a convenient wrapper to set annotations on the snapshot regarding its content type.
func (in *Snapshot) SetContentType(t string) {
	if in.Annotations == nil {
		in.Annotations = make(map[string]string)
	}
	in.Annotations[ContentTypeKey] = t
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
