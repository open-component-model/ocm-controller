// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package configdata

import (
	"github.com/xeipuuv/gojsonschema"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConfigData defines configuration options.
// This data is not promoted to being a CRD, but it contains versionable properties.
// The following is an example structure of this data:
/*
apiVersion: config.ocm.software/v1alpha1
kind: ConfigData
metadata:
  name: ocm-config
configuration:
  defaults:
    replicas: 1
  rules:
  - value: (( replicas ))
    file: helm_release.yaml
    path: spec.values.replicaCount
  schema:
    type: object
    additionalProperties: false
    properties:
      replicas:
        type: string
localization:
- file: helm_release.yaml
  tag: spec.chart.spec.version
  resource:
    name: chart
- file: helm_repository.yaml
  mapping:
    path: spec.url
    transform: |-
          package main

          import (
            "encoding/json"
            "path"
          )

          result: string

          for x in component.resources {
            if x.name == "chart" {
              result: path.Dir(x.access.imageReference)
            }
          }

          out: json.Marshal("oci://"+result)
*/
// Localization and Configuration are both provided in the same struct. This is to minimize
// duplication and having to learn multiple structures and Kubernetes Objects.
// ConfigData is not a full-fledged Kubernetes object because nothing is reconciling it
// and there is no need for the cluster to be aware of its presence. It's meant to be created
// and maintained by the Component Consumer.
// Various configuration and localization methods are available to the consumer:
// - **plain yaml substitution**
// - **cue lang** ( https://cuelang.org/ ) with a playground (https://cuelang.org/play/)
// - **strategic patch merge**
// The available Localization resource properties are:
// - **image**
// - **repository**
// - **registry**
// - **tag**.
type ConfigData struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Configuration     ConfigurationSpec  `json:"configuration,omitempty"`
	Localization      []LocalizationRule `json:"localization,omitempty"`
}

type ConfigurationSpec struct {
	Defaults map[string]any      `json:"defaults"`
	Schema   gojsonschema.Schema `json:"schema"`
	Rules    []ConfigRule        `json:"rules"`
}

type ConfigRule struct {
	Value any    `json:"value"`
	Path  string `json:"path"`
	File  string `json:"file"`
}

type LocalizationRule struct {
	Resource   ResourceItem `json:"resource"`
	File       string       `json:"file"`
	Registry   string       `json:"registry,omitempty"`
	Mapping    *Mapping     `json:"mapping,omitempty"`
	Repository string       `json:"repository,omitempty"`
	Image      string       `json:"image,omitempty"`
	Tag        string       `json:"tag,omitempty"`
}

type Mapping struct {
	Path      string `json:"path"`
	Transform string `json:"transform"`
}

type ResourceItem struct {
	Name          string            `json:"name"`
	ExtraIdentity map[string]string `json:"extraIdentity,omitempty"`
}
