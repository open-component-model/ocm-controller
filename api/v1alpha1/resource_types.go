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
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ResourceSpec defines the desired state of Resource
type ResourceSpec struct {
	// +required
	Interval metav1.Duration `json:"interval"`

	// +required
	ComponentRef meta.NamespacedObjectReference `json:"componentRef"`

	// Resource names a Source that this Resource watches.
	// +required
	Resource string `json:"resource"`
}

// GetRequeueAfter returns the duration after which the Resource must be
// reconciled again.
func (in Resource) GetRequeueAfter() time.Duration {
	return in.Spec.Interval.Duration
}

// ResourceStatus defines the observed state of Resource
type ResourceStatus struct {
	// Ready denotes the state of processing a Source.
	Ready bool `json:"ready"`
	// Snapshot is a snapshot of a Source in the in-cluster OCI storage.
	Snapshot string `json:"snapshot"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Resource is the Schema for the resources API
type Resource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ResourceSpec   `json:"spec,omitempty"`
	Status ResourceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ResourceList contains a list of Resource
type ResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Resource `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Resource{}, &ResourceList{})
}
