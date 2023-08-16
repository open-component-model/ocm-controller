// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"strings"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	wasmapi "github.com/open-component-model/ocm-controller/pkg/wasm/api/v1alpha1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ResourcePipelineSpec defines the desired state of ResourcePipeline
type ResourcePipelineSpec struct {
	// +required
	Interval metav1.Duration `json:"interval"`

	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// +required
	SourceRef ResourcePipelineSource `json:"sourceRef"`

	// +optional
	Secrets []ResourcePipelineSecretSpec `json:"secrets,omitempty"`

	// +optional
	Parameters *apiextensionsv1.JSON `json:"parameters,omitempty"`

	// +optional
	PipelineSpec *PipelineSpec `json:"pipelineSpec,omitempty"`

	// +optional
	DeliverySpec *DeliverySpec `json:"deliverySpec,omitempty"`
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
	Name string `json:"name"`

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
	// +kubebuilder:validation:Pattern="^([A-Za-z0-9\\.\\/]+):(v[0-9\\.]+)@([a-z]+)$"
	// +required
	Module string `json:"module"`

	// +kubebuilder:default="ghcr.io/open-component-model/delivery"
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
	DeployerInventories map[string]*wasmapi.ResourceInventory `json:"deployerInventories,omitempty"`
}

// GetRequeueAfter returns the duration after which the Resource should be reconciled.
func (in ResourcePipeline) GetRequeueAfter() time.Duration {
	return in.Spec.Interval.Duration
}

func (in ResourcePipeline) GetInventory(step string) *wasmapi.ResourceInventory {
	result, ok := in.Status.DeployerInventories[step]
	if !ok {
		return nil
	}
	return result
}

func (in *ResourcePipeline) SetInventory(step string, inventory *wasmapi.ResourceInventory) {
	if in.Status.DeployerInventories == nil {
		in.Status.DeployerInventories = make(map[string]*wasmapi.ResourceInventory)
	}
	in.Status.DeployerInventories[step] = inventory
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

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
