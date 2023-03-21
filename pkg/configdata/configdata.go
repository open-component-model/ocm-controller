// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package configdata

import (
	"github.com/xeipuuv/gojsonschema"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ConfigData struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Configuration     ConfigurationSpec  `json:"configuration,omitempty"`
	Localization      []LocalizationRule `json:"localization,omitempty"`
}

type ConfigurationSpec struct {
	Defaults map[string]string   `json:"defaults"`
	Schema   gojsonschema.Schema `json:"schema"`
	Rules    []ConfigRule        `json:"rules"`
}

type ConfigRule struct {
	Value string `json:"value"`
	Path  string `json:"path"`
	// File in case of a data stream is an optional value.
	// +optional
	File string `json:"file,omitempty"`
}

type LocalizationRule struct {
	Resource ResourceItem `json:"resource"`
	// File in case of a plain data stream is an optional value.
	// +optional
	File       string   `json:"file,omitempty"`
	Registry   string   `json:"registry,omitempty"`
	Mapping    *Mapping `json:"mapping,omitempty"`
	Repository string   `json:"repository,omitempty"`
	Image      string   `json:"image,omitempty"`
	Tag        string   `json:"tag,omitempty"`
}

type Mapping struct {
	Path      string `json:"path"`
	Transform string `json:"transform"`
}

type ResourceItem struct {
	Name          string            `json:"name"`
	ExtraIdentity map[string]string `json:"extraIdentity,omitempty"`
}
