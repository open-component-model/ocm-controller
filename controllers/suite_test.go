// Copyright 2022.
// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

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
				Name:    "test-resource",
				Version: "v0.0.1",
				ReferencePath: []map[string]string{
					{
						"name": "test",
					},
				},
			},
			SnapshotTemplate: v1alpha1.SnapshotTemplateSpec{
				Name: "snapshot-test-name",
				Tag:  "v0.0.1",
			},
		},
	}
)

type CreateComponentVersionOption struct {
	componentOverride *v1alpha1.ComponentVersion
}

type CreateComponentVersionOptionFunc func(opts *CreateComponentVersionOption)

func WithComponentVersionOverrides(cv *v1alpha1.ComponentVersion) CreateComponentVersionOptionFunc {
	return func(opts *CreateComponentVersionOption) {
		opts.componentOverride = cv
	}
}

func (t *testEnv) CreateComponentVersion(opts ...CreateComponentVersionOptionFunc) (*v1alpha1.ComponentVersion, error) {
	component := DefaultComponent
	defaultOpts := &CreateComponentVersionOption{}
	for _, opt := range opts {
		opt(defaultOpts)
	}
	if defaultOpts.componentOverride == nil {
		return component, nil
	}

	return mergeObjects(component, defaultOpts.componentOverride)
}

type CreateResourceOption struct {
	resourceOverride *v1alpha1.Resource
}

type CreateResourceOptionFunc func(opts *CreateResourceOption)

func WithCreateResourceOverrides(r *v1alpha1.Resource) CreateResourceOptionFunc {
	return func(opts *CreateResourceOption) {
		opts.resourceOverride = r
	}
}

func (t *testEnv) CreateResource(opts ...CreateResourceOptionFunc) (*v1alpha1.Resource, error) {
	resource := DefaultResource
	defaultOpts := &CreateResourceOption{}
	for _, opt := range opts {
		opt(defaultOpts)
	}
	if defaultOpts.resourceOverride == nil {
		return resource, nil
	}

	return mergeObjects(resource, defaultOpts.resourceOverride)
}

func mergeObjects[T runtime.Object](defaultObj, override T) (T, error) {
	var result T
	// merge the override with the default component
	b, err := json.Marshal(override)
	if err != nil {
		return result, fmt.Errorf("failed to encode override to JSON: %w", err)
	}
	m := make(map[string]any)
	if err := json.Unmarshal(b, &m); err != nil {
		return result, fmt.Errorf("failed to decode component: %w", err)
	}
	// the map is cleaned of empty values, it's ready to be merged with component
	removeEmptyFields(m)

	b, err = json.Marshal(m)
	if err != nil {
		return result, fmt.Errorf("failed to marshal map: %w", err)
	}

	if err := json.Unmarshal(b, &defaultObj); err != nil {
		return result, fmt.Errorf("failed to override component: %w", err)
	}

	return defaultObj, nil
}

func removeEmptyFields(m map[string]any) {
	// do this as long as there are items that have been deleted.
	for k, v := range m {
		if v == nil {
			delete(m, k)
		}
		switch vv := v.(type) {
		case map[string]any:
			if len(vv) == 0 {
				delete(m, k)
			} else {
				removeEmptyFields(vv)
			}
		case string:
			if vv == "" || v == "0s" {
				delete(m, k)
			}
		case int:
			if vv == 0 {
				delete(m, k)
			}
		case any:
			if vv == nil {
				delete(m, k)
			}
		}
	}

	// cleanup
	for k, v := range m {
		if vv, ok := v.(map[string]any); ok {
			if len(vv) == 0 {
				delete(m, k)
			}
		}
		if v == nil {
			delete(m, k)
		}
	}
}

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
