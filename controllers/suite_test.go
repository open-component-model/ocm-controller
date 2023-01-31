// Copyright 2022.
// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"testing"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	ocmmetav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/ocm.software/v3alpha1"
	ocmruntime "github.com/open-component-model/ocm/pkg/runtime"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
)

type testEnv struct {
	scheme *runtime.Scheme
	obj    []client.Object
}

// FakeKubeClientOption defines options to construct a fake kube client. There are some defaults involved.
// Scheme gets corev1 and v1alpha1 schemes by default. Anything that is passed in will override current
// defaults.
type FakeKubeClientOption func(testEnv *testEnv)

// WithScheme provides an option to set the scheme.
func WithScheme(scheme *runtime.Scheme) FakeKubeClientOption {
	return func(testEnv *testEnv) {
		testEnv.scheme = scheme
	}
}

// WithObjects provides an option to set objects for the fake client.
func WithObjets(obj ...client.Object) FakeKubeClientOption {
	return func(testEnv *testEnv) {
		testEnv.obj = obj
	}
}

// FakeKubeClient creates a fake kube client with some defaults and optional arguments.
func (t *testEnv) FakeKubeClient(opts ...FakeKubeClientOption) client.Client {
	for _, o := range opts {
		o(t)
	}
	return fake.NewClientBuilder().WithScheme(t.scheme).WithObjects(t.obj...).Build()
}

var (
	DefaultComponent = &v1alpha1.ComponentVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-component",
			Namespace: "default",
		},
		Spec: v1alpha1.ComponentVersionSpec{
			Interval:  metav1.Duration{Duration: 10 * time.Minute},
			Component: "github.com/open-component-model/test-component",
			Version: v1alpha1.Version{
				Semver: "v0.0.1",
			},
			Repository: v1alpha1.Repository{
				URL: "github.com/open-component-model/test",
			},
			Verify: []v1alpha1.Signature{},
			References: v1alpha1.ReferencesConfig{
				Expand: true,
			},
		},
	}
	DefaultResource = &v1alpha1.Resource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-resource",
			Namespace: "default",
		},
		Spec: v1alpha1.ResourceSpec{
			Interval: metav1.Duration{Duration: 10 * time.Minute},
			ComponentVersionRef: meta.NamespacedObjectReference{
				Name:      "test-component",
				Namespace: "default",
			},
			Resource: v1alpha1.ResourceRef{
				Name:    "introspect-image",
				Version: "1.0.0",
				ReferencePath: []map[string]string{
					{
						"name": "test",
					},
				},
			},
			SnapshotTemplate: v1alpha1.SnapshotTemplateSpec{
				Name: "snapshot-test-name",
			},
		},
	}
	DefaultComponentDescriptor = &v1alpha1.ComponentDescriptor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DefaultComponent.Name + "-descriptor",
			Namespace: DefaultComponent.Namespace,
		},
		Spec: v1alpha1.ComponentDescriptorSpec{
			ComponentVersionSpec: v3alpha1.ComponentVersionSpec{
				Resources: []v3alpha1.Resource{
					{
						ElementMeta: v3alpha1.ElementMeta{
							Name:    "introspect-image",
							Version: "1.0.0",
						},
						Type:     "ociImage",
						Relation: "local",
						Access: &ocmruntime.UnstructuredTypedObject{
							Object: map[string]interface{}{
								"globalAccess": map[string]interface{}{
									"digest":    "sha256:7f0168496f273c1e2095703a050128114d339c580b0906cd124a93b66ae471e2",
									"mediaType": "application/vnd.docker.distribution.manifest.v2+tar+gzip",
									"ref":       "ghcr.io/mandelsoft/cnudie/component-descriptors/github.com/vasu1124/introspect",
									"size":      29047129,
									"type":      "ociBlob",
								},
								"localReference": "sha256:7f0168496f273c1e2095703a050128114d339c580b0906cd124a93b66ae471e2",
								"mediaType":      "application/vnd.docker.distribution.manifest.v2+tar+gzip",
								"type":           "localBlob",
							},
						},
						Digest: &ocmmetav1.DigestSpec{
							HashAlgorithm:          "sha256",
							NormalisationAlgorithm: "ociArtifactDigest/v1",
							Value:                  "6a1c7637a528ab5957ab60edf73b5298a0a03de02a96be0313ee89b22544840c",
						},
					},
				},
			},
			Version: "v0.0.1",
		},
		Status: v1alpha1.ComponentDescriptorStatus{},
	}

	DefaultLocalization = &v1alpha1.Localization{
		TypeMeta: metav1.TypeMeta{
			Kind: "Localization",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-localization",
			Namespace: "default",
		},
		Spec: v1alpha1.MutationSpec{
			Interval: metav1.Duration{},
			ComponentVersionRef: meta.NamespacedObjectReference{
				Name:      DefaultComponent.Name,
				Namespace: DefaultComponent.Namespace,
			},
			ConfigRef: v1alpha1.ConfigReference{
				Resource: v1alpha1.Source{
					ResourceRef: &v1alpha1.ResourceRef{
						Name: DefaultResource.Name,
					},
				},
			},
			SnapshotTemplate: v1alpha1.SnapshotTemplateSpec{
				Name: "test-localization-modified",
			},
		},
	}
	DefaultConfiguration = &v1alpha1.Configuration{
		TypeMeta: metav1.TypeMeta{
			Kind: "Localization",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-configuration",
			Namespace: "default",
		},
		Spec: v1alpha1.MutationSpec{
			Interval: metav1.Duration{},
			ComponentVersionRef: meta.NamespacedObjectReference{
				Name:      DefaultComponent.Name,
				Namespace: DefaultComponent.Namespace,
			},
			ConfigRef: v1alpha1.ConfigReference{
				Resource: v1alpha1.Source{
					ResourceRef: &v1alpha1.ResourceRef{
						Name: DefaultResource.Name,
					},
				},
			},
			Values: map[string]string{
				"message": "this is a new message",
				"color":   "bittersweet",
			},
			SnapshotTemplate: v1alpha1.SnapshotTemplateSpec{
				Name: "test-configuration-modified",
			},
		},
	}
)

var env *testEnv

func TestMain(m *testing.M) {
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	env = &testEnv{
		scheme: scheme,
	}
	m.Run()
}
