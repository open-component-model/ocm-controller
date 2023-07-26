// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"github.com/open-component-model/ocm/v2/pkg/contexts/ocm/compdesc/versions/ocm.software/v3alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ComponentDescriptorStatus defines the observed state of ComponentDescriptor
type ComponentDescriptorStatus struct {
}

// ComponentDescriptorSpec adds a version to the top level component descriptor definition.
type ComponentDescriptorSpec struct {
	v3alpha1.ComponentVersionSpec `json:",inline"`
	Version                       string `json:"version"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:shortName=cd
//+kubebuilder:subresource:status

// ComponentDescriptor is the Schema for the componentdescriptors API
type ComponentDescriptor struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ComponentDescriptorSpec   `json:"spec,omitempty"`
	Status ComponentDescriptorStatus `json:"status,omitempty"`
}

// GetResource return a given resource in a component descriptor if it exists.
func (in ComponentDescriptor) GetResource(name string) *v3alpha1.Resource {
	for _, r := range in.Spec.Resources {
		if r.Name != name {
			continue
		}
		return &r
	}
	return nil
}

//+kubebuilder:object:root=true

// ComponentDescriptorList contains a list of ComponentDescriptor
type ComponentDescriptorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComponentDescriptor `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ComponentDescriptor{}, &ComponentDescriptorList{})
}
