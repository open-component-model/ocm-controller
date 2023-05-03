// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	ocmdesc "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ComponentVersionKind is the string representation of a ComponentVersion.
	ComponentVersionKind = "ComponentVersion"
)

// ComponentVersionSpec defines the desired state of ComponentVersion
type ComponentVersionSpec struct {
	// +required
	Interval metav1.Duration `json:"interval"`

	// Every Component Version has a name.
	// Name and version are the identifier for a Component Version and therefor for the artifact set described by it.
	// A component name SHOULD reference a location where the component’s resources (typically source code, and/or documentation) are hosted.
	// It MUST be a DNS compliant name with lowercase characters and MUST contain a name after the domain.
	// Examples:
	// - github.com/pathToYourRepo
	// +required
	Component string `json:"component"`

	// Component versions refer to specific snapshots of a component. A common scenario being the release of a component.
	// +required
	Version Version `json:"version"`

	// +required
	Repository Repository `json:"repository"`

	// +optional
	Verify []Signature `json:"verify,omitempty"`

	// +optional
	References ReferencesConfig `json:"references,omitempty"`

	// Suspend stops all operations on this component version object.
	// +optional
	Suspend bool `json:"suspend,omitempty"`

	// ServiceAccountName can be used to configure access to both destination and source repositories.
	// If service account is defined, it's usually redundant to define access to either source or destination, but
	// it is still allowed to do so.
	// https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#add-imagepullsecrets-to-a-service-account
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
}

// Repository defines the OCM Repository.
type Repository struct {
	//TODO@souleb: do we need a scheme for the url?
	// add description for each field
	// Do we need a type field? (e.g. oci, git, s3, etc.)
	URL string `json:"url"`

	// +optional
	SecretRef *v1.LocalObjectReference `json:"secretRef,omitempty"`
}

// SecretRefValue clearly denotes that the requested option is a Secret.
type SecretRefValue struct {
	SecretRef v1.LocalObjectReference `json:"secretRef"`
}

// Signature defines the details of a signature to use for verification.
type Signature struct {
	// Name of the signature.
	Name string `json:"name"`

	// Key which is used for verification.
	PublicKey SecretRefValue `json:"publicKey"`
}

// Version defines version upgrade / downgrade options.
type Version struct {
	// +optional
	Semver string `json:"semver,omitempty"`

	// +optional
	AllowRollback bool `json:"allowRollback,omitempty"`
}

type ReferencesConfig struct {
	// +optional
	Expand bool `json:"expand,omitempty"`
}

// Reference contains all referred components and their versions.
type Reference struct {
	// +required
	Name string `json:"name"`

	// +required
	Version string `json:"version"`

	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	References []Reference `json:"references,omitempty"`

	// +optional
	ExtraIdentity map[string]string `json:"extraIdentity,omitempty"`

	// +optional
	ComponentDescriptorRef meta.NamespacedObjectReference `json:"componentDescriptorRef,omitempty"`
}

// ComponentVersionStatus defines the observed state of ComponentVersion
type ComponentVersionStatus struct {
	// ObservedGeneration is the last reconciled generation.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// +optional
	ComponentDescriptor Reference `json:"componentDescriptor,omitempty"`

	// +optional
	ReconciledVersion string `json:"reconciledVersion,omitempty"`

	// +optional
	Verified bool `json:"verified,omitempty"`
}

// GetVersion returns the reconciled version for the component
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

// LookupReferenceForIdentity returns the reference that matches up with the given identity selector.
func (in ComponentVersion) LookupReferenceForIdentity(key ocmdesc.IdentitySelector) Reference {
	// Loop through the reference struct in References and return the reference that matches with the
	// given ExtraIdentity.
	return Reference{}
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:shortName=cv
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description=""
//+kubebuilder:printcolumn:name="Version",type="string",JSONPath=".status.reconciledVersion",description=""
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description=""
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message",description=""

// ComponentVersion is the Schema for the ComponentVersions API
type ComponentVersion struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ComponentVersionSpec   `json:"spec,omitempty"`
	Status ComponentVersionStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ComponentVersionList contains a list of ComponentVersion
type ComponentVersionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComponentVersion `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ComponentVersion{}, &ComponentVersionList{})
}
