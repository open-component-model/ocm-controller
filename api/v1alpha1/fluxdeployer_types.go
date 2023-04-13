// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FluxDeployerSpec defines the desired state of FluxDeployer
type FluxDeployerSpec struct {
	// +required
	SourceRef ObjectReference `json:"sourceRef"`

	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	KustomizationTemplate kustomizev1.KustomizationSpec `json:"kustomizationTemplate"`
}

// +kubebuilder:resource:shortName=fd
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description=""
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message",description=""

// FluxDeployerStatus defines the observed state of FluxDeployer
type FluxDeployerStatus struct {
	// ObservedGeneration is the last reconciled generation.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// +optional
	Kustomization string `json:"kustomization"`

	// +optional
	OCIRepository string `json:"ociRepository"`
}

// GetConditions returns the conditions of the ComponentVersion.
func (in *FluxDeployer) GetConditions() []metav1.Condition {
	return in.Status.Conditions
}

// SetConditions sets the conditions of the ComponentVersion.
func (in *FluxDeployer) SetConditions(conditions []metav1.Condition) {
	in.Status.Conditions = conditions
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// FluxDeployer is the Schema for the fluxdeployers API
type FluxDeployer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FluxDeployerSpec   `json:"spec,omitempty"`
	Status FluxDeployerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// FluxDeployerList contains a list of FluxDeployer
type FluxDeployerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FluxDeployer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FluxDeployer{}, &FluxDeployerList{})
}
