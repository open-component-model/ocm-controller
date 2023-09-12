// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"strings"
	"time"

	esv1beta1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1beta1"
	"github.com/fluxcd/pkg/apis/meta"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ResourcePipelineSpec defines the desired state of ResourcePipeline
type ResourcePipelineSpec struct {
	// +required
	Interval metav1.Duration `json:"interval"`

	// +optional
	Suspend bool `json:"suspend,omitempty"`

	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// +required
	SourceRef ObjectReference `json:"sourceRef"`

	// +optional
	Secrets map[string]ResourcePipelineSecretSpec `json:"secrets,omitempty"`

	// +optional
	Parameters *apiextensionsv1.JSON `json:"parameters,omitempty"`

	// +optional
	PipelineSpec *PipelineSpec `json:"pipelineSpec,omitempty"`
}

// ResourcePipelineSource defines the component version and resource
// which will be processed by the pipeline.
type ResourcePipelineSource struct {
	// +required
	Name string `json:"name"`

	// +optional
	Namespace string `json:"namespace,omitempty"`

	// +required
	Resource string `json:"resource"`
}

// ResourcePipelineSecretSpec specifies access to a secret resource
// that can be used within either the pipeline or delivery stages.
type ResourcePipelineSecretSpec struct {
	// +required
	RemoteRef esv1beta1.ExternalSecretDataRemoteRef `json:"remoteRef"`

	// +required
	SecretStoreRef meta.NamespacedObjectReference `json:"secretStoreRef"`
}

// PipelineSpec holds the steps that constitute the pipeline.
type PipelineSpec struct {
	// +required
	Steps []WasmStep `json:"steps"`
}

// DeliverySpec holds a set of targets onto which the pipeline output will be deployed.
type DeliverySpec struct {
	// +required
	Targets []WasmStep `json:"targets"`
}

// WasmStep defines the name version and location of a wasm module that is stored// in an ocm component. The format of the module name must be <component-name>:<component-version>@<resource-name>. Optionally a registry address can be specified.
type WasmStep struct {
	// +required
	Name string `json:"name"`

	// +kubebuilder:example="ocm.software/modules:v1.0.0@kustomizer"
	// +kubebuilder:validation:Pattern="^([A-Za-z0-9\\.\\/]+):(v[0-9\\.\\-a-z]+)@([a-z]+)$"
	// +required
	Module string `json:"module"`

	// +kubebuilder:default="ghcr.io/open-component-model"
	// +optional
	Registry string `json:"registry,omitempty"`

	// +optional
	Values *apiextensionsv1.JSON `json:"values,omitempty"`
}

func (w WasmStep) GetComponent() string {
	return strings.Split(w.Module, ":")[0]
}

func (w WasmStep) GetComponentVersion() string {
	p1 := strings.Split(w.Module, ":")[1]
	return strings.Split(p1, "@")[0]
}

func (w WasmStep) GetResource() string {
	return strings.Split(w.Module, "@")[1]
}

// ResourcePipelineStatus defines the observed state of ResourcePipeline
type ResourcePipelineStatus struct {
	// ObservedGeneration is the last reconciled generation.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// +optional
	LatestSnapshotDigest string `json:"latestSnapshotDigest,omitempty"`

	// +optional
	SnapshotName string `json:"snapshotName,omitempty"`
}

// GetRequeueAfter returns the duration after which the Resource should be reconciled.
func (in ResourcePipeline) GetRequeueAfter() time.Duration {
	return in.Spec.Interval.Duration
}

// GetSnapshotDigest returns the digest of the Resource's associated Snapshot.
func (in ResourcePipeline) GetSnapshotDigest() string {
	return in.Status.LatestSnapshotDigest
}

// GetSnapshotName returns the name of the Resource's associated Snapshot.
func (in ResourcePipeline) GetSnapshotName() string {
	return in.Status.SnapshotName
}

// GetConditions returns the conditions of the ComponentVersion.
func (in *ResourcePipeline) GetConditions() []metav1.Condition {
	return in.Status.Conditions
}

// SetConditions sets the conditions of the ComponentVersion.
func (in *ResourcePipeline) SetConditions(conditions []metav1.Condition) {
	in.Status.Conditions = conditions
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=rp
//+kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description=""
//+kubebuilder:printcolumn:name="Digest",type="string",JSONPath=".status.latestSnapshotDigest",description=""
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description=""

// ResourcePipeline is the Schema for the resourcepipelines API
type ResourcePipeline struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ResourcePipelineSpec   `json:"spec,omitempty"`
	Status ResourcePipelineStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ResourcePipelineList contains a list of ResourcePipeline
type ResourcePipelineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResourcePipeline `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ResourcePipeline{}, &ResourcePipelineList{})
}
