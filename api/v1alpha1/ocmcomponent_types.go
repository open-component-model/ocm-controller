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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SecretRef is a reference to a secret used to access the OCI repository.
type SecretRef struct {
	Name string `json:"name"`
}

// Repository defines the OCM Repository.
type Repository struct {
	URL       string    `json:"url"`
	SecretRef SecretRef `json:"secretRef"`
}

// Verify holds the secret which contains the signing and verification keys.
type Verify struct {
	SecretRef SecretRef `json:"secretRef"`
}

// OCMComponentSpec defines the desired state of OCMComponent
type OCMComponentSpec struct {
	Interval   time.Duration `json:"interval"`
	Name       string        `json:"name"`
	Version    string        `json:"version"`
	Repository Repository    `json:"repository"`
	Verify     Verify        `json:"verify"`
}

// OCMComponentStatus defines the observed state of OCMComponent
type OCMComponentStatus struct {
	ComponentDescriptor string `json:"componentDescriptor"`
	// TODO: DeployPackage could be a configMap....
	DeployPackage string `json:"deployPackage"`
	Verified      bool   `json:"verified"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// OCMComponent is the Schema for the ocmcomponents API
type OCMComponent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OCMComponentSpec   `json:"spec,omitempty"`
	Status OCMComponentStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OCMComponentList contains a list of OCMComponent
type OCMComponentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OCMComponent `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OCMComponent{}, &OCMComponentList{})
}
