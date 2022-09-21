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
	"github.com/fluxcd/pkg/apis/meta"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClassResource defines the class resource that has been generated for the given component.
type ClassResource struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// Overrides define user given values to be applied to package defined values in the Class.
type Overrides struct {
	Actions apiextensionsv1.JSON `json:"actions"`
}

// WorkflowSpec defines the desired state of Workflow
type WorkflowSpec struct {
	ComponentRef   meta.NamespacedObjectReference `json:"componentRef"`
	ClassResource  ClassResource                  `json:"classResource"`
	Overrides      Overrides                      `json:"overrides"`
	ServiceAccount string                         `json:"serviceAccount"`
}

// WorkflowStatus defines the observed state of Workflow
type WorkflowStatus struct {
	Ready    bool   `json:"ready"`
	Snapshot string `json:"snapshot"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Workflow is the Schema for the actions API
type Workflow struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkflowSpec   `json:"spec,omitempty"`
	Status WorkflowStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// WorkflowList contains a list of Workflow
type WorkflowList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Workflow `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Workflow{}, &WorkflowList{})
}
