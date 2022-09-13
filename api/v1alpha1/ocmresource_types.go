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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OCMResourceSpec defines the desired state of OCMResource
type OCMResourceSpec struct {
	// Resource names a Source that this OCMResource watches.
	Resource string `json:"resource"`
}

// OCMResourceStatus defines the observed state of OCMResource
type OCMResourceStatus struct {
	// Ready denotes the state of processing a Source.
	Ready bool `json:"ready"`
	// Snapshot is a snapshot of a Source in the in-cluster OCI storage.
	Snapshot string `json:"snapshot"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// OCMResource is the Schema for the ocmresources API
type OCMResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OCMResourceSpec   `json:"spec,omitempty"`
	Status OCMResourceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OCMResourceList contains a list of OCMResource
type OCMResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OCMResource `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OCMResource{}, &OCMResourceList{})
}
