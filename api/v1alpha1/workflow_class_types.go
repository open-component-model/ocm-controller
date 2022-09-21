/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Provider defines the provider of this workflow.
type Provider struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
}

// Stage is a single stage in the list of workflows.
type Stage struct {
	Provider Provider `json:"provider"`
	// +kubebuilder:validation:Enum:=Source;Action
	Type string               `json:"type"`
	Spec apiextensionsv1.JSON `json:"spec"`
}

// WorkflowItem defines the workflow section which depicts the order of processing of the given resources.
type WorkflowItem struct {
	Input string `json:"input,omitempty"`
	Name  string `json:"name"`
}

// WorkflowClassSpec defines the desired state of WorkflowClass
type WorkflowClassSpec struct {
	Stages   map[string]Stage `json:"stages"`
	Workflow []WorkflowItem   `json:"workflow"`
}

// WorkflowClassStatus defines the observed state of WorkflowClass
type WorkflowClassStatus struct {
	Ready    bool   `json:"ready"`
	Snapshot string `json:"snapshot"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// WorkflowClass is the Schema for the actions API
type WorkflowClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkflowClassSpec   `json:"spec,omitempty"`
	Status WorkflowClassStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// WorkflowClassList contains a list of WorkflowClass
type WorkflowClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WorkflowClass `json:"items"`
}

func init() {
	SchemeBuilder.Register(&WorkflowClass{}, &WorkflowClassList{})
}
