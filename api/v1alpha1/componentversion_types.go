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

// ComponentVersionSpec defines the desired state of ComponentVersion
type ComponentVersionSpec struct {
	Interval   time.Duration    `json:"interval,omitempty"`
	Name       string           `json:"name,omitempty"`
	Version    string           `json:"version,omitempty"`
	Repository Repository       `json:"repository,omitempty"`
	Verify     Verify           `json:"verify,omitempty"`
	References ReferencesConfig `json:"references,omitempty"`
}

type ReferencesConfig struct {
	Expand bool `json:"expand,omitempty"`
}

// ComponentVersionStatus defines the observed state of ComponentVersion
type ComponentVersionStatus struct {
	ComponentDescriptor string `json:"componentDescriptor"`
	// TODO: DeployPackage could be a configMap....
	DeployPackage string `json:"deployPackage"`
	Verified      bool   `json:"verified"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

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
