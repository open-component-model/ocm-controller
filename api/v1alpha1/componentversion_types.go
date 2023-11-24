// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"fmt"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ComponentVersionKind is the string representation of a ComponentVersion.
	ComponentVersionKind = "ComponentVersion"
)

// ComponentVersionSpec specifies the configuration required to retrieve a
// component descriptor for a component version.
type ComponentVersionSpec struct {
	// Component specifies the name of the ComponentVersion.
	// +required
	Component string `json:"component"`

	// Version specifies the version information for the ComponentVersion.
	// +required
	Version Version `json:"version"`

	// Repository provides details about the OCI repository from which the component
	// descriptor can be retrieved.
	// +required
	Repository Repository `json:"repository"`

	// Interval specifies the interval at which the Repository will be checked for updates.
	// +required
	Interval metav1.Duration `json:"interval"`

	// Verify specifies a list signatures that should be validated before the ComponentVersion
	// is marked Verified.
	// +optional
	Verify []Signature `json:"verify,omitempty"`

	// References specifies configuration for the handling of nested component references.
	// +optional
	References ReferencesConfig `json:"references,omitempty"`

	// Suspend can be used to temporarily pause the reconciliation of the ComponentVersion resource.
	// +optional
	Suspend bool `json:"suspend,omitempty"`

	// ServiceAccountName can be used to configure access to both destination and source repositories.
	// If service account is defined, it's usually redundant to define access to either source or destination, but
	// it is still allowed to do so.
	// https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#add-imagepullsecrets-to-a-service-account
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
}

// Repository specifies access details for the repository that contains OCM ComponentVersions.
type Repository struct {
	// URL specifies the URL of the OCI registry in which the ComponentVersion is stored.
	// MUST NOT CONTAIN THE SCHEME.
	// +required
	URL string `json:"url"`

	// SecretRef specifies the credentials used to access the OCI registry.
	// +optional
	SecretRef *v1.LocalObjectReference `json:"secretRef,omitempty"`
}

// Signature defines the details of a signature to use for verification.
type Signature struct {
	// Name specifies the name of the signature. An OCM component may have multiple
	// signatures.
	Name string `json:"name"`

	// PublicKey provides a reference to a Kubernetes Secret that contains a public key
	// which will be used to validate the named signature.
	// +optional
	PublicKey *SecretRef `json:"publicKey,omitempty"`

	// PublicKeyBlob a public key blob encoded in base64 for verification.
	// +optional
	PublicKeyBlob []byte `json:"publicKeyBlob,omitempty"`
}

// SecretRef specifies a reference to a Secret.
type SecretRef struct {
	SecretRef v1.LocalObjectReference `json:"secretRef"`
}

// Version specifies version information that can be used to resolve a Component Version.
type Version struct {
	// Semver specifies a semantic version constraint for the Component Version.
	// +optional
	Semver string `json:"semver,omitempty"`
}

// ReferencesConfig specifies how component references should be handled when reconciling
// the root component.
type ReferencesConfig struct {
	// Expand specifies if a Kubernetes API resource of kind ComponentDescriptor should
	// be generated for each component reference that is present in the root ComponentVersion.
	// +optional
	Expand bool `json:"expand,omitempty"`
}

// Reference contains all referred components and their versions.
type Reference struct {
	// Name specifies the name of the referenced component.
	// +required
	Name string `json:"name"`

	// Version specifies the version of the referenced component.
	// +required
	Version string `json:"version"`

	// References is a list of component references.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	References []Reference `json:"references,omitempty"`

	// ExtraIdentity specifies additional identity attributes of the referenced component.
	// +optional
	ExtraIdentity map[string]string `json:"extraIdentity,omitempty"`

	// ComponentDescriptorRef specifies the reference for the Kubernetes object representing
	// the ComponentDescriptor.
	// +optional
	ComponentDescriptorRef meta.NamespacedObjectReference `json:"componentDescriptorRef,omitempty"`
}

// ComponentVersionStatus defines the observed state of ComponentVersion.
type ComponentVersionStatus struct {
	// ObservedGeneration is the last reconciled generation.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions holds the conditions for the ComponentVersion.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ComponentDescriptor holds the ComponentDescriptor information for the ComponentVersion.
	// +optional
	ComponentDescriptor Reference `json:"componentDescriptor,omitempty"`

	// ReconciledVersion is a string containing the version of the latest reconciled ComponentVersion.
	// +optional
	ReconciledVersion string `json:"reconciledVersion,omitempty"`

	// Verified is a boolean indicating whether all the specified signatures have been verified and are valid.
	// +optional
	Verified bool `json:"verified,omitempty"`
}

func (in *ComponentVersion) GetVID() map[string]string {
	vid := fmt.Sprintf("%s:%s", in.Status.ComponentDescriptor.Name, in.Status.ReconciledVersion)
	metadata := make(map[string]string)
	metadata[GroupVersion.Group+"/component_version"] = vid

	return metadata
}

func (in *ComponentVersion) SetObservedGeneration(v int64) {
	in.Status.ObservedGeneration = v
}

// GetComponentName returns the name of the component.
func (in *ComponentVersion) GetComponentName() string {
	return in.Spec.Component
}

// GetVersion returns the reconciled version for the component.
func (in *ComponentVersion) GetVersion() string {
	return in.Status.ReconciledVersion
}

// GetConditions returns the conditions of the ComponentVersion.
func (in *ComponentVersion) GetConditions() []metav1.Condition {
	return in.Status.Conditions
}

// SetConditions sets the conditions of the ComponentVersion.
func (in *ComponentVersion) SetConditions(conditions []metav1.Condition) {
	in.Status.Conditions = conditions
}

// GetRequeueAfter returns the duration after which the ComponentVersion must be
// reconciled again.
func (in ComponentVersion) GetRequeueAfter() time.Duration {
	return in.Spec.Interval.Duration
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:shortName=cv
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description=""
//+kubebuilder:printcolumn:name="Version",type="string",JSONPath=".status.reconciledVersion",description=""
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description=""
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message",description=""

// ComponentVersion is the Schema for the ComponentVersions API.
type ComponentVersion struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ComponentVersionSpec   `json:"spec,omitempty"`
	Status ComponentVersionStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ComponentVersionList contains a list of ComponentVersion.
type ComponentVersionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComponentVersion `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ComponentVersion{}, &ComponentVersionList{})
}
