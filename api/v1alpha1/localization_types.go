// Copyright 2022.
// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Localization is the Schema for the localizations API
type Localization struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MutationSpec   `json:"spec,omitempty"`
	Status MutationStatus `json:"status,omitempty"`
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
