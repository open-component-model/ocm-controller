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
	"k8s.io/apimachinery/pkg/types"
)

// LocalizationSpec defines the desired state of Localization
type LocalizationSpec struct {
	// +required
	Interval metav1.Duration `json:"interval"`

	// +required
	SourceRef meta.NamespacedObjectKindReference `json:"sourceRef"`

	// +required
	ConfigRef ConfigReference `json:"configRef"`

	// +required
	SnapshotTemplate SnapshotTemplateSpec `json:"snapshotTemplate"`
}

type ConfigReference struct {
	// +required
	ComponentVersionRef meta.NamespacedObjectReference `json:"componentRef"`

	// +required
	Resource ResourceRef `json:"resource"`
}

type ReferencePath struct {
	// +required
	Name string `json:"name"`
}

type ResourceRef struct {
	// +required
	Name string `json:"name"`

	// +optional
	ExtraIdentity map[string]string `json:"extraIdentity,omitempty"`

	// +optional
	// TODO: This should be a list of names, for now to keep it simple, we restrict it to a single item.
	ReferencePath ReferencePath `json:"referencePath,omitempty"`
}

// LocalizationStatus defines the observed state of Localization
type LocalizationStatus struct {
	// ObservedGeneration is the last reconciled generation.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// +optional
	LatestSnapshotDigest string `json:"latestSnapshotDigest,omitempty"`

	LatestConfigVersion string `json:"latestConfigVersion,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Localization is the Schema for the localizations API
type Localization struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LocalizationSpec   `json:"spec,omitempty"`
	Status LocalizationStatus `json:"status,omitempty"`
}

func (in Localization) GetSourceSnapshotKey() types.NamespacedName {
	return types.NamespacedName{
		Namespace: in.Spec.SourceRef.Namespace,
		Name:      in.Spec.SourceRef.Name,
	}
}

// GetRequeueAfter returns the duration after which the Localization must be
// reconciled again.
func (in Localization) GetRequeueAfter() time.Duration {
	return in.Spec.Interval.Duration
}

//+kubebuilder:object:root=true

// LocalizationList contains a list of Localization
type LocalizationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Localization `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Localization{}, &LocalizationList{})
}
