// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	"github.com/onsi/gomega/ghttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	ocmmetav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	cachefakes "github.com/open-component-model/ocm-controller/pkg/cache/fakes"
	"github.com/open-component-model/ocm-controller/pkg/ocm/fakes"
	ocmsnapshot "github.com/open-component-model/ocm-controller/pkg/snapshot"
)

var configurationConfigData = []byte(`kind: ConfigData
metadata:
  name: test-config-data
  namespace: default
configuration:
  defaults:
    color: red
    message: Hello, world!
  schema:
    type: object
    additionalProperties: false
    properties:
      color:
        type: string
      message:
        type: string
  rules:
  - value: (( message ))
    file: configmap.yaml
    path: data.PODINFO_UI_MESSAGE
  - value: (( color ))
    file: configmap.yaml
    path: data.PODINFO_UI_COLOR
`)

type configurationTestCase struct {
	name                string
	mock                func(fakeCache *cachefakes.FakeCache, fakeOcm *fakes.MockFetcher)
	componentVersion    func() *v1alpha1.ComponentVersion
	componentDescriptor func() *v1alpha1.ComponentDescriptor
	snapshot            func(cv *v1alpha1.ComponentVersion, resource *v1alpha1.Resource) *v1alpha1.Snapshot
	source              func(snapshot *v1alpha1.Snapshot) v1alpha1.ObjectReference
	expectError         string
	expectedContent     map[string]string
}

func TestConfigurationReconciler(t *testing.T) {
	testCases := []configurationTestCase{
		{
			name: "with snapshot as a source",
			componentVersion: func() *v1alpha1.ComponentVersion {
				cv := DefaultComponent.DeepCopy()
				cv.Status.ReconciledVersion = "v0.0.1"
				cv.Status.ObservedGeneration = 5
				cv.Status.ComponentDescriptor = v1alpha1.Reference{
					Name:    "test-component",
					Version: "v0.0.1",
					ComponentDescriptorRef: meta.NamespacedObjectReference{
						Name:      cv.Name + "-descriptor",
						Namespace: cv.Namespace,
					},
				}
				return cv
			},
			componentDescriptor: func() *v1alpha1.ComponentDescriptor {
				return DefaultComponentDescriptor.DeepCopy()
			},
			source: func(snapshot *v1alpha1.Snapshot) v1alpha1.ObjectReference {
				return v1alpha1.ObjectReference{
					NamespacedObjectKindReference: meta.NamespacedObjectKindReference{
						APIVersion: v1alpha1.GroupVersion.String(),
						Kind:       "Resource",
						Name:       "test-resource",
						Namespace:  snapshot.Namespace,
					},
				}
			},
			snapshot: func(cv *v1alpha1.ComponentVersion, resource *v1alpha1.Resource) *v1alpha1.Snapshot {
				name := "resource-snapshot"
				identity := ocmmetav1.Identity{
					v1alpha1.ComponentNameKey:    cv.Status.ComponentDescriptor.ComponentDescriptorRef.Name,
					v1alpha1.ComponentVersionKey: cv.Status.ComponentDescriptor.Version,
					v1alpha1.ResourceNameKey:     resource.Spec.SourceRef.ResourceRef.Name,
					v1alpha1.ResourceVersionKey:  resource.Spec.SourceRef.ResourceRef.Version,
				}
				sourceSnapshot := &v1alpha1.Snapshot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: cv.Namespace,
					},
					Spec: v1alpha1.SnapshotSpec{
						Identity: identity,
					},
				}
				resource.Status.SnapshotName = name
				return sourceSnapshot
			},
			mock: func(fakeCache *cachefakes.FakeCache, fakeOcm *fakes.MockFetcher) {
				content, err := os.Open(filepath.Join("testdata", "configuration-map.tar"))
				require.NoError(t, err)
				fakeCache.FetchDataByDigestReturns(content, nil)
				fakeOcm.GetResourceReturns(io.NopCloser(bytes.NewBuffer(configurationConfigData)), "", nil)
				cmp := getMockComponent(DefaultComponent)
				fakeOcm.GetComponentVersionReturnsForName(cmp.GetName(), cmp, nil)
			},
			expectedContent: map[string]string{"PODINFO_UI_MESSAGE": "this is a new message", "PODINFO_UI_COLOR": "bittersweet"},
		},
		{
			name: "add new node that does not exist",
			componentVersion: func() *v1alpha1.ComponentVersion {
				cv := DefaultComponent.DeepCopy()
				cv.Status.ReconciledVersion = "v0.0.1"
				cv.Status.ObservedGeneration = 5
				cv.Status.ComponentDescriptor = v1alpha1.Reference{
					Name:    "test-component",
					Version: "v0.0.1",
					ComponentDescriptorRef: meta.NamespacedObjectReference{
						Name:      cv.Name + "-descriptor",
						Namespace: cv.Namespace,
					},
				}
				return cv
			},
			componentDescriptor: func() *v1alpha1.ComponentDescriptor {
				return DefaultComponentDescriptor.DeepCopy()
			},
			source: func(snapshot *v1alpha1.Snapshot) v1alpha1.ObjectReference {
				return v1alpha1.ObjectReference{
					NamespacedObjectKindReference: meta.NamespacedObjectKindReference{
						APIVersion: v1alpha1.GroupVersion.String(),
						Kind:       "Resource",
						Name:       "test-resource",
						Namespace:  snapshot.Namespace,
					},
				}
			},
			snapshot: func(cv *v1alpha1.ComponentVersion, resource *v1alpha1.Resource) *v1alpha1.Snapshot {
				name := "resource-snapshot"
				identity := ocmmetav1.Identity{
					v1alpha1.ComponentNameKey:    cv.Status.ComponentDescriptor.ComponentDescriptorRef.Name,
					v1alpha1.ComponentVersionKey: cv.Status.ComponentDescriptor.Version,
					v1alpha1.ResourceNameKey:     resource.Spec.SourceRef.ResourceRef.Name,
					v1alpha1.ResourceVersionKey:  resource.Spec.SourceRef.ResourceRef.Version,
				}
				sourceSnapshot := &v1alpha1.Snapshot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: cv.Namespace,
					},
					Spec: v1alpha1.SnapshotSpec{
						Identity: identity,
					},
				}
				resource.Status.SnapshotName = name
				return sourceSnapshot
			},
			mock: func(fakeCache *cachefakes.FakeCache, fakeOcm *fakes.MockFetcher) {
				content, err := os.Open(filepath.Join("testdata", "configuration-map.tar"))
				require.NoError(t, err)
				fakeCache.FetchDataByDigestReturns(content, nil)
				testConfigData := []byte(`kind: ConfigData
metadata:
  name: test-config-data
  namespace: default
configuration:
  defaults:
    color: red
    message: Hello, world!
    newValue: This is a new value!
  schema:
    type: object
    additionalProperties: false
    properties:
      color:
        type: string
      message:
        type: string
      newValue:
        type: string
  rules:
  - value: (( message ))
    file: configmap.yaml
    path: data.PODINFO_UI_MESSAGE
  - value: (( color ))
    file: configmap.yaml
    path: data.PODINFO_UI_COLOR
  - value: (( newValue ))
    file: configmap.yaml
    path: data.PODINFO_NEW_VALUE
`)
				fakeOcm.GetResourceReturns(io.NopCloser(bytes.NewBuffer(testConfigData)), "", nil)
				cmp := getMockComponent(DefaultComponent)
				fakeOcm.GetComponentVersionReturnsForName(cmp.GetName(), cmp, nil)
			},
			expectedContent: map[string]string{"PODINFO_UI_MESSAGE": "this is a new message", "PODINFO_UI_COLOR": "bittersweet", "PODINFO_NEW_VALUE": "This is a new value!"},
		},
		{
			name: "values without defaults are not ignored",
			componentVersion: func() *v1alpha1.ComponentVersion {
				cv := DefaultComponent.DeepCopy()
				cv.Status.ReconciledVersion = "v0.0.1"
				cv.Status.ObservedGeneration = 5
				cv.Status.ComponentDescriptor = v1alpha1.Reference{
					Name:    "test-component",
					Version: "v0.0.1",
					ComponentDescriptorRef: meta.NamespacedObjectReference{
						Name:      cv.Name + "-descriptor",
						Namespace: cv.Namespace,
					},
				}
				return cv
			},
			componentDescriptor: func() *v1alpha1.ComponentDescriptor {
				return DefaultComponentDescriptor.DeepCopy()
			},
			source: func(snapshot *v1alpha1.Snapshot) v1alpha1.ObjectReference {
				return v1alpha1.ObjectReference{
					NamespacedObjectKindReference: meta.NamespacedObjectKindReference{
						APIVersion: v1alpha1.GroupVersion.String(),
						Kind:       "Resource",
						Name:       "test-resource",
						Namespace:  snapshot.Namespace,
					},
				}
			},
			snapshot: func(cv *v1alpha1.ComponentVersion, resource *v1alpha1.Resource) *v1alpha1.Snapshot {
				name := "resource-snapshot"
				identity := ocmmetav1.Identity{
					v1alpha1.ComponentNameKey:    cv.Status.ComponentDescriptor.ComponentDescriptorRef.Name,
					v1alpha1.ComponentVersionKey: cv.Status.ComponentDescriptor.Version,
					v1alpha1.ResourceNameKey:     resource.Spec.SourceRef.ResourceRef.Name,
					v1alpha1.ResourceVersionKey:  resource.Spec.SourceRef.ResourceRef.Version,
				}
				sourceSnapshot := &v1alpha1.Snapshot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: cv.Namespace,
					},
					Spec: v1alpha1.SnapshotSpec{
						Identity: identity,
					},
				}
				resource.Status.SnapshotName = name
				return sourceSnapshot
			},
			mock: func(fakeCache *cachefakes.FakeCache, fakeOcm *fakes.MockFetcher) {
				content, err := os.Open(filepath.Join("testdata", "configuration-map.tar"))
				require.NoError(t, err)
				fakeCache.FetchDataByDigestReturns(content, nil)
				testConfigData := []byte(`kind: ConfigData
metadata:
  name: test-config-data
  namespace: default
configuration:
  defaults:
    color: red
    message: Hello, world!
  schema:
    type: object
    additionalProperties: false
    properties:
      color:
        type: string
      message:
        type: string
  rules:
  - value: (( message ))
    file: configmap.yaml
    path: data.PODINFO_UI_MESSAGE
  - value: (( color ))
    file: configmap.yaml
    path: data.PODINFO_UI_COLOR
  - value: this is a new value
    file: configmap.yaml
    path: data.PODINFO_NEW_VALUE
`)
				fakeOcm.GetResourceReturns(io.NopCloser(bytes.NewBuffer(testConfigData)), "", nil)
				cmp := getMockComponent(DefaultComponent)
				fakeOcm.GetComponentVersionReturnsForName(cmp.GetName(), cmp, nil)
			},
			expectedContent: map[string]string{"PODINFO_UI_MESSAGE": "this is a new message", "PODINFO_UI_COLOR": "bittersweet", "PODINFO_NEW_VALUE": "this is a new value"},
		},
		{
			name: "with resource as a source",
			componentVersion: func() *v1alpha1.ComponentVersion {
				cv := DefaultComponent.DeepCopy()
				cv.Status.ObservedGeneration = 5
				cv.Status.ComponentDescriptor = v1alpha1.Reference{
					Name:    "test-component",
					Version: "v0.0.1",
					ComponentDescriptorRef: meta.NamespacedObjectReference{
						Name:      cv.Name + "-descriptor",
						Namespace: cv.Namespace,
					},
				}
				return cv
			},
			componentDescriptor: func() *v1alpha1.ComponentDescriptor {
				return DefaultComponentDescriptor.DeepCopy()
			},
			snapshot: func(cv *v1alpha1.ComponentVersion, resource *v1alpha1.Resource) *v1alpha1.Snapshot {
				// do nothing
				return nil
			},
			source: func(snapshot *v1alpha1.Snapshot) v1alpha1.ObjectReference {
				cv := DefaultComponent.DeepCopy()
				return v1alpha1.ObjectReference{
					NamespacedObjectKindReference: meta.NamespacedObjectKindReference{
						APIVersion: v1alpha1.GroupVersion.String(),
						Kind:       "ComponentVersion",
						Name:       cv.Name,
						Namespace:  cv.Namespace,
					},
					ResourceRef: &v1alpha1.ResourceReference{
						ElementMeta: v1alpha1.ElementMeta{
							Name:    "some-resource",
							Version: "1.0.0",
						},
					},
				}
			},
			mock: func(fakeCache *cachefakes.FakeCache, fakeOcm *fakes.MockFetcher) {
				content, err := os.Open(filepath.Join("testdata", "configuration-map.tar"))
				require.NoError(t, err)
				fakeOcm.GetResourceReturnsOnCall(0, content, nil)
				fakeOcm.GetResourceReturnsOnCall(1, io.NopCloser(bytes.NewBuffer(configurationConfigData)), nil)
			},
			expectedContent: map[string]string{"PODINFO_UI_MESSAGE": "this is a new message", "PODINFO_UI_COLOR": "bittersweet"},
		},
		{
			name:        "expect error when get resource fails with snapshot",
			expectError: "failed to fetch resource data from snapshot: failed to fetch data: boo",
			componentVersion: func() *v1alpha1.ComponentVersion {
				cv := DefaultComponent.DeepCopy()
				cv.Status.ObservedGeneration = 5
				cv.Status.ComponentDescriptor = v1alpha1.Reference{
					Name:    "test-component",
					Version: "v0.0.1",
					ComponentDescriptorRef: meta.NamespacedObjectReference{
						Name:      cv.Name + "-descriptor",
						Namespace: cv.Namespace,
					},
				}
				return cv
			},
			componentDescriptor: func() *v1alpha1.ComponentDescriptor {
				return DefaultComponentDescriptor.DeepCopy()
			},
			source: func(snapshot *v1alpha1.Snapshot) v1alpha1.ObjectReference {
				return v1alpha1.ObjectReference{
					NamespacedObjectKindReference: meta.NamespacedObjectKindReference{
						APIVersion: v1alpha1.GroupVersion.String(),
						Kind:       "Resource",
						Name:       "test-resource",
						Namespace:  snapshot.Namespace,
					},
				}
			},
			snapshot: func(cv *v1alpha1.ComponentVersion, resource *v1alpha1.Resource) *v1alpha1.Snapshot {
				name := "test-snapshot"
				identity := ocmmetav1.Identity{
					v1alpha1.ComponentNameKey:    cv.Status.ComponentDescriptor.ComponentDescriptorRef.Name,
					v1alpha1.ComponentVersionKey: cv.Status.ComponentDescriptor.Version,
					v1alpha1.ResourceNameKey:     resource.Spec.SourceRef.ResourceRef.Name,
					v1alpha1.ResourceVersionKey:  resource.Spec.SourceRef.ResourceRef.Version,
				}
				snapshot := &v1alpha1.Snapshot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: cv.Namespace,
					},
					Spec: v1alpha1.SnapshotSpec{
						Identity: identity,
					},
				}
				resource.Status.SnapshotName = name
				return snapshot
			},
			mock: func(fakeCache *cachefakes.FakeCache, fakeOcm *fakes.MockFetcher) {
				fakeCache.FetchDataByDigestReturns(nil, errors.New("boo"))
			},
		},
		{
			name:        "expect error when get resource fails without snapshots",
			expectError: "failed to fetch resource from component version: boo",
			componentVersion: func() *v1alpha1.ComponentVersion {
				cv := DefaultComponent.DeepCopy()
				cv.Status.ObservedGeneration = 5
				cv.Status.ComponentDescriptor = v1alpha1.Reference{
					Name:    "test-component",
					Version: "v0.0.1",
					ComponentDescriptorRef: meta.NamespacedObjectReference{
						Name:      cv.Name + "-descriptor",
						Namespace: cv.Namespace,
					},
				}
				return cv
			},
			componentDescriptor: func() *v1alpha1.ComponentDescriptor {
				return DefaultComponentDescriptor.DeepCopy()
			},
			snapshot: func(cv *v1alpha1.ComponentVersion, resource *v1alpha1.Resource) *v1alpha1.Snapshot {
				// do nothing
				return nil
			},
			source: func(snapshot *v1alpha1.Snapshot) v1alpha1.ObjectReference {
				cv := DefaultComponent.DeepCopy()
				return v1alpha1.ObjectReference{
					NamespacedObjectKindReference: meta.NamespacedObjectKindReference{
						APIVersion: v1alpha1.GroupVersion.String(),
						Kind:       "ComponentVersion",
						Name:       cv.Name,
						Namespace:  cv.Namespace,
					},
					ResourceRef: &v1alpha1.ResourceReference{
						ElementMeta: v1alpha1.ElementMeta{
							Name:    "some-resource",
							Version: "1.0.0",
						},
					},
				}
			},
			mock: func(fakeCache *cachefakes.FakeCache, fakeOcm *fakes.MockFetcher) {
				fakeOcm.GetResourceReturns(nil, "digest", errors.New("boo"))
			},
		},
		{
			name:        "get resource fails during config data fetch",
			expectError: "failed to get data for config ref: failed to fetch resource data from resource ref: failed to fetch resource from component version: ",
			componentVersion: func() *v1alpha1.ComponentVersion {
				cv := DefaultComponent.DeepCopy()
				cv.Status.ObservedGeneration = 5
				cv.Status.ComponentDescriptor = v1alpha1.Reference{
					Name:    "test-component",
					Version: "v0.0.1",
					ComponentDescriptorRef: meta.NamespacedObjectReference{
						Name:      cv.Name + "-descriptor",
						Namespace: cv.Namespace,
					},
				}

				return cv
			},
			componentDescriptor: func() *v1alpha1.ComponentDescriptor {
				return DefaultComponentDescriptor.DeepCopy()
			},
			snapshot: func(cv *v1alpha1.ComponentVersion, resource *v1alpha1.Resource) *v1alpha1.Snapshot {
				// do nothing
				return nil
			},
			source: func(snapshot *v1alpha1.Snapshot) v1alpha1.ObjectReference {
				cv := DefaultComponent.DeepCopy()
				return v1alpha1.ObjectReference{
					NamespacedObjectKindReference: meta.NamespacedObjectKindReference{
						APIVersion: v1alpha1.GroupVersion.String(),
						Kind:       "ComponentVersion",
						Name:       cv.Name,
						Namespace:  cv.Namespace,
					},
					ResourceRef: &v1alpha1.ResourceReference{
						ElementMeta: v1alpha1.ElementMeta{
							Name:    "some-resource",
							Version: "1.0.0",
						},
					},
				}
			},
			mock: func(fakeCache *cachefakes.FakeCache, fakeOcm *fakes.MockFetcher) {
				content, err := os.Open(filepath.Join("testdata", "configuration-map.tar"))
				require.NoError(t, err)
				fakeOcm.GetResourceReturnsOnCall(0, content, nil)
				fakeOcm.GetResourceReturnsOnCall(1, nil, errors.New("boo"))
			},
		},
		{
			name:        "error while running configurator",
			expectError: "failed to apply config ref: failed to configure resource: configurator error: error while doing cascade with: processing template ocmAdjustmentsTemplateKey: unresolved nodes:\n\t(( nope ))\tin template ocmAdjustmentsTemplateKey\tocmAdjustmentsTemplateKey.[0].value\t(ocmAdjustmentsTemplateKey.name:subst-0.value)\t*'nope' not found",
			componentVersion: func() *v1alpha1.ComponentVersion {
				cv := DefaultComponent.DeepCopy()
				cv.Status.ObservedGeneration = 5
				cv.Status.ComponentDescriptor = v1alpha1.Reference{
					Name:    "test-component",
					Version: "v0.0.1",
					ComponentDescriptorRef: meta.NamespacedObjectReference{
						Name:      cv.Name + "-descriptor",
						Namespace: cv.Namespace,
					},
				}

				return cv
			},
			componentDescriptor: func() *v1alpha1.ComponentDescriptor {
				return DefaultComponentDescriptor.DeepCopy()
			},
			snapshot: func(cv *v1alpha1.ComponentVersion, resource *v1alpha1.Resource) *v1alpha1.Snapshot {
				// do nothing
				return nil
			},
			source: func(snapshot *v1alpha1.Snapshot) v1alpha1.ObjectReference {
				cv := DefaultComponent.DeepCopy()
				return v1alpha1.ObjectReference{
					NamespacedObjectKindReference: meta.NamespacedObjectKindReference{
						APIVersion: v1alpha1.GroupVersion.String(),
						Kind:       "ComponentVersion",
						Name:       cv.Name,
						Namespace:  cv.Namespace,
					},
					ResourceRef: &v1alpha1.ResourceReference{
						ElementMeta: v1alpha1.ElementMeta{
							Name:    "some-resource",
							Version: "1.0.0",
						},
					},
				}
			},
			mock: func(fakeCache *cachefakes.FakeCache, fakeOcm *fakes.MockFetcher) {
				content, err := os.Open(filepath.Join("testdata", "configuration-map.tar"))
				require.NoError(t, err)
				fakeOcm.GetResourceReturnsOnCall(0, content, nil)
				testConfigData := []byte(`kind: ConfigData
metadata:
  name: test-config-data
  namespace: default
configuration:
  defaults:
    color: red
    message: Hello, world!
  schema:
    type: object
    additionalProperties: false
    properties:
      color:
        type: string
      message:
        type: string
  rules:
  - value: (( nope ))
    file: configmap.yaml
    path: data.PODINFO_UI_MESSAGE
  - value: (( color ))
    file: configmap.yaml
    path: data.PODINFO_UI_COLOR
`)
				fakeOcm.GetResourceReturnsOnCall(1, io.NopCloser(bytes.NewBuffer(testConfigData)), nil)
			},
		},
		{
			name:        "configuration fails because the file does not exist",
			expectError: "no such file or directory",
			componentVersion: func() *v1alpha1.ComponentVersion {
				cv := DefaultComponent.DeepCopy()
				cv.Status.ObservedGeneration = 5
				cv.Status.ComponentDescriptor = v1alpha1.Reference{
					Name:    "test-component",
					Version: "v0.0.1",
					ComponentDescriptorRef: meta.NamespacedObjectReference{
						Name:      cv.Name + "-descriptor",
						Namespace: cv.Namespace,
					},
				}

				return cv
			},
			componentDescriptor: func() *v1alpha1.ComponentDescriptor {
				return DefaultComponentDescriptor.DeepCopy()
			},
			snapshot: func(cv *v1alpha1.ComponentVersion, resource *v1alpha1.Resource) *v1alpha1.Snapshot {
				// do nothing
				return nil
			},
			source: func(snapshot *v1alpha1.Snapshot) v1alpha1.ObjectReference {
				cv := DefaultComponent.DeepCopy()
				return v1alpha1.ObjectReference{
					NamespacedObjectKindReference: meta.NamespacedObjectKindReference{
						APIVersion: v1alpha1.GroupVersion.String(),
						Kind:       "ComponentVersion",
						Name:       cv.Name,
						Namespace:  cv.Namespace,
					},
					ResourceRef: &v1alpha1.ResourceReference{
						ElementMeta: v1alpha1.ElementMeta{
							Name:    "some-resource",
							Version: "1.0.0",
						},
					},
				}
			},
			mock: func(fakeCache *cachefakes.FakeCache, fakeOcm *fakes.MockFetcher) {
				content, err := os.Open(filepath.Join("testdata", "configuration-map.tar"))
				require.NoError(t, err)
				fakeOcm.GetResourceReturnsOnCall(0, content, nil)
				testConfigData := []byte(`kind: ConfigData
metadata:
  name: test-config-data
  namespace: default
configuration:
  defaults:
    color: red
    message: Hello, world!
  schema:
    type: object
    additionalProperties: false
    properties:
      color:
        type: string
      message:
        type: string
  rules:
  - value: (( message ))
    file: nope.yaml
    path: data.PODINFO_UI_MESSAGE
  - value: (( color ))
    file: configmap.yaml
    path: data.PODINFO_UI_COLOR
`)
				fakeOcm.GetResourceReturnsOnCall(1, io.NopCloser(bytes.NewBuffer(testConfigData)), nil)
			},
		},
		{
			name:        "it fails to marshal the config data",
			expectError: "failed to unmarshal content",
			componentVersion: func() *v1alpha1.ComponentVersion {
				cv := DefaultComponent.DeepCopy()
				cv.Status.ObservedGeneration = 5
				cv.Status.ComponentDescriptor = v1alpha1.Reference{
					Name:    "test-component",
					Version: "v0.0.1",
					ComponentDescriptorRef: meta.NamespacedObjectReference{
						Name:      cv.Name + "-descriptor",
						Namespace: cv.Namespace,
					},
				}

				return cv
			},
			componentDescriptor: func() *v1alpha1.ComponentDescriptor {
				return DefaultComponentDescriptor.DeepCopy()
			},
			snapshot: func(cv *v1alpha1.ComponentVersion, resource *v1alpha1.Resource) *v1alpha1.Snapshot {
				// do nothing
				return nil
			},
			source: func(snapshot *v1alpha1.Snapshot) v1alpha1.ObjectReference {
				cv := DefaultComponent.DeepCopy()
				return v1alpha1.ObjectReference{
					NamespacedObjectKindReference: meta.NamespacedObjectKindReference{
						APIVersion: v1alpha1.GroupVersion.String(),
						Kind:       "ComponentVersion",
						Name:       cv.Name,
						Namespace:  cv.Namespace,
					},
					ResourceRef: &v1alpha1.ResourceReference{
						ElementMeta: v1alpha1.ElementMeta{
							Name:    "some-resource",
							Version: "1.0.0",
						},
					},
				}
			},
			mock: func(fakeCache *cachefakes.FakeCache, fakeOcm *fakes.MockFetcher) {
				content, err := os.Open(filepath.Join("testdata", "localization-deploy.tar"))
				require.NoError(t, err)
				fakeOcm.GetResourceReturnsOnCall(0, content, nil)
				testConfigData := []byte(`iaminvalidyaml`)
				fakeOcm.GetResourceReturnsOnCall(1, io.NopCloser(bytes.NewBuffer(testConfigData)), nil)
			},
		},
		{
			name:        "it fails to push data to the cache",
			expectError: "failed to push blob to local registry: boo",
			componentVersion: func() *v1alpha1.ComponentVersion {
				cv := DefaultComponent.DeepCopy()
				cv.Status.ObservedGeneration = 5
				cv.Status.ComponentDescriptor = v1alpha1.Reference{
					Name:    "test-component",
					Version: "v0.0.1",
					ComponentDescriptorRef: meta.NamespacedObjectReference{
						Name:      cv.Name + "-descriptor",
						Namespace: cv.Namespace,
					},
				}

				return cv
			},
			componentDescriptor: func() *v1alpha1.ComponentDescriptor {
				return DefaultComponentDescriptor.DeepCopy()
			},
			snapshot: func(cv *v1alpha1.ComponentVersion, resource *v1alpha1.Resource) *v1alpha1.Snapshot {
				// do nothing
				return nil
			},
			source: func(snapshot *v1alpha1.Snapshot) v1alpha1.ObjectReference {
				cv := DefaultComponent.DeepCopy()
				return v1alpha1.ObjectReference{
					NamespacedObjectKindReference: meta.NamespacedObjectKindReference{
						APIVersion: v1alpha1.GroupVersion.String(),
						Kind:       "ComponentVersion",
						Name:       cv.Name,
						Namespace:  cv.Namespace,
					},
					ResourceRef: &v1alpha1.ResourceReference{
						ElementMeta: v1alpha1.ElementMeta{
							Name:    "some-resource",
							Version: "1.0.0",
						},
					},
				}
			},
			mock: func(fakeCache *cachefakes.FakeCache, fakeOcm *fakes.MockFetcher) {
				content, err := os.Open(filepath.Join("testdata", "configuration-map.tar"))
				require.NoError(t, err)
				fakeCache.PushDataReturns("", errors.New("boo"))
				fakeOcm.GetResourceReturnsOnCall(0, content, nil)
				fakeOcm.GetResourceReturnsOnCall(1, io.NopCloser(bytes.NewBuffer(configurationConfigData)), nil)
			},
		},
	}
	for i, tt := range testCases {
		t.Run(fmt.Sprintf("%d: %s", i, tt.name), func(t *testing.T) {
			cv := tt.componentVersion()
			conditions.MarkTrue(cv, meta.ReadyCondition, meta.SucceededReason, "test")

			cd := tt.componentDescriptor()
			resource := DefaultResource.DeepCopy()
			snapshot := tt.snapshot(cv, resource)
			source := tt.source(snapshot)

			configuration := DefaultConfiguration.DeepCopy()
			configuration.Spec.SourceRef = source
			configuration.Status.SnapshotName = "configuration-snapshot"

			objs := []client.Object{cv, cd, resource, configuration}

			if snapshot != nil {
				conditions.MarkTrue(snapshot, meta.ReadyCondition, meta.SucceededReason, "test")
				objs = append(objs, snapshot)
			}

			client := env.FakeKubeClient(WithObjects(objs...), WithAddToScheme(sourcev1.AddToScheme))
			dynClient := env.FakeDynamicKubeClient(WithObjects(objs...))
			cache := &cachefakes.FakeCache{}
			snapshotWriter := ocmsnapshot.NewOCIWriter(client, cache, env.scheme)
			fakeOcm := &fakes.MockFetcher{}
			recorder := record.NewFakeRecorder(32)
			tt.mock(cache, fakeOcm)

			cr := ConfigurationReconciler{
				Client:        client,
				DynamicClient: dynClient,
				Scheme:        env.scheme,
				EventRecorder: recorder,
				MutationReconciler: MutationReconcileLooper{
					Client:         client,
					DynamicClient:  dynClient,
					Scheme:         env.scheme,
					OCMClient:      fakeOcm,
					Cache:          cache,
					SnapshotWriter: snapshotWriter,
				},
			}

			t.Log("reconciling configuration")
			_, err := cr.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: configuration.Namespace,
					Name:      configuration.Name,
				},
			})

			getErr := client.Get(context.Background(), types.NamespacedName{
				Namespace: configuration.Namespace,
				Name:      configuration.Name,
			}, configuration)
			require.NoError(t, getErr)

			if tt.expectError != "" {
				require.ErrorContains(t, err, tt.expectError)
				assert.True(t, conditions.IsFalse(configuration, meta.ReadyCondition))

				return
			}

			require.NoError(t, err)

			snapshotOutput := &v1alpha1.Snapshot{}
			err = client.Get(context.Background(), types.NamespacedName{
				Namespace: configuration.Namespace,
				Name:      configuration.Status.SnapshotName,
			}, snapshotOutput)

			t.Log("check if target snapshot has been created and cache was called")
			require.NoError(t, err)

			t.Log("extracting the passed in data and checking if the configuration worked")
			args := cache.PushDataCallingArgumentsOnCall(0)
			assert.Equal(t, "sha-5540475038233850640", args.Name)
			assert.Equal(t, "1.0.0", args.Version)
			sourceFile := extractFileFromTarGz(t, io.NopCloser(bytes.NewBuffer([]byte(args.Content))), "configmap.yaml")
			configMap := corev1.ConfigMap{}
			assert.NoError(t, yaml.Unmarshal(sourceFile, &configMap))
			assert.Equal(t, tt.expectedContent, configMap.Data)

			err = client.Get(context.Background(), types.NamespacedName{
				Namespace: configuration.Namespace,
				Name:      configuration.Name,
			}, configuration)
			require.NoError(t, err)

			assert.True(t, conditions.IsTrue(configuration, meta.ReadyCondition))

			close(recorder.Events)
			event := ""
			for e := range recorder.Events {
				if strings.Contains(e, "Reconciliation finished, next run in") {
					event = e

					break
				}
			}
			assert.Contains(t, event, "Reconciliation finished, next run in")
		})
	}
}

func TestConfigurationValuesFrom(t *testing.T) {
	testCases := []struct {
		name          string
		configuration func(source client.Object) *v1alpha1.Configuration
		setup         func() client.Object
	}{
		{
			name: "configuration values from GitRepository",
			configuration: func(source client.Object) *v1alpha1.Configuration {
				configuration := DefaultConfiguration.DeepCopy()
				configuration.Status.SnapshotName = "configuration-snapshot"
				configuration.Spec.ValuesFrom = &v1alpha1.ValuesSource{
					FluxSource: &v1alpha1.FluxValuesSource{
						SourceRef: meta.NamespacedObjectKindReference{
							Kind:      "GitRepository",
							Name:      source.GetName(),
							Namespace: source.GetNamespace(),
						},
						Path:    "config/values.yaml",
						SubPath: "test.backend",
					},
				}
				configuration.Spec.Values = nil

				return configuration
			},
			setup: func() client.Object {
				path := "/file.tar.gz"
				server := ghttp.NewServer()
				server.RouteToHandler("GET", path, func(writer http.ResponseWriter, request *http.Request) {
					http.ServeFile(writer, request, "testdata/git-repo.tar.gz")
				})
				checksum := "87670827f3d1a10094e3226381c95168b6ce92344ac1a1c2345caaeb7cc6b7d8"
				gitRepo := createGitRepository("patch-repo", "default", server.URL()+path, checksum)
				return gitRepo
			},
		},
		{
			name: "configuration values from ConfigMap",
			configuration: func(source client.Object) *v1alpha1.Configuration {
				configuration := DefaultConfiguration.DeepCopy()
				configuration.Status.SnapshotName = "configuration-snapshot"
				configuration.Spec.ValuesFrom = &v1alpha1.ValuesSource{
					ConfigMapSource: &v1alpha1.ConfigMapSource{
						SourceRef: meta.LocalObjectReference{
							Name: "test-config-data",
						},
						Key:     "values.yaml",
						SubPath: "test.backend",
					},
				}
				configuration.Spec.Values = nil

				return configuration
			},
			setup: func() client.Object {
				valuesFile, err := os.ReadFile("testdata/values.yaml")
				require.NoError(t, err)
				configMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config-data",
						Namespace: "default",
					},
					Data: map[string]string{
						"values.yaml": string(valuesFile),
					},
				}

				return configMap
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cv := DefaultComponent.DeepCopy()
			conditions.MarkTrue(cv, meta.ReadyCondition, meta.SucceededReason, "test")

			cd := DefaultComponentDescriptor.DeepCopy()
			resource := DefaultResource.DeepCopy()
			name := "test-snapshot"
			identity := ocmmetav1.Identity{
				v1alpha1.ComponentNameKey:    cv.Status.ComponentDescriptor.ComponentDescriptorRef.Name,
				v1alpha1.ComponentVersionKey: cv.Status.ComponentDescriptor.Version,
				v1alpha1.ResourceNameKey:     resource.Spec.SourceRef.ResourceRef.Name,
				v1alpha1.ResourceVersionKey:  resource.Spec.SourceRef.ResourceRef.Version,
			}
			snapshot := &v1alpha1.Snapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: cv.Namespace,
				},
				Spec: v1alpha1.SnapshotSpec{
					Identity: identity,
				},
			}
			conditions.MarkTrue(snapshot, meta.ReadyCondition, meta.SucceededReason, "test")

			resource.Status.SnapshotName = name

			source := v1alpha1.ObjectReference{
				NamespacedObjectKindReference: meta.NamespacedObjectKindReference{
					APIVersion: v1alpha1.GroupVersion.String(),
					Kind:       "Resource",
					Name:       "test-resource",
					Namespace:  snapshot.Namespace,
				},
			}

			configSource := tc.setup()
			objs := []client.Object{cv, cd, resource, configSource}

			configuration := tc.configuration(configSource)
			configuration.Spec.SourceRef = source
			objs = append(objs, configuration, snapshot)

			client := env.FakeKubeClient(WithObjects(objs...), WithAddToScheme(sourcev1.AddToScheme))
			dynClient := env.FakeDynamicKubeClient(WithObjects(objs...))
			cache := &cachefakes.FakeCache{}
			snapshotWriter := ocmsnapshot.NewOCIWriter(client, cache, env.scheme)
			fakeOcm := &fakes.MockFetcher{}
			recorder := record.NewFakeRecorder(32)
			content, err := os.Open(filepath.Join("testdata", "configuration-map.tar"))
			require.NoError(t, err)
			cache.FetchDataByDigestReturns(content, nil)
			fakeOcm.GetResourceReturns(io.NopCloser(bytes.NewBuffer(configurationConfigData)), "", nil)
			cmp := getMockComponent(DefaultComponent)
			fakeOcm.GetComponentVersionReturnsForName(cmp.GetName(), cmp, nil)

			cr := ConfigurationReconciler{
				Client:        client,
				DynamicClient: dynClient,
				Scheme:        env.scheme,
				EventRecorder: recorder,
				MutationReconciler: MutationReconcileLooper{
					Client:         client,
					DynamicClient:  dynClient,
					Scheme:         env.scheme,
					OCMClient:      fakeOcm,
					Cache:          cache,
					SnapshotWriter: snapshotWriter,
				},
			}

			t.Log("reconciling configuration")
			_, err = cr.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: configuration.Namespace,
					Name:      configuration.Name,
				},
			})
			require.NoError(t, err)

			getErr := client.Get(context.Background(), types.NamespacedName{
				Namespace: configuration.Namespace,
				Name:      configuration.Name,
			}, configuration)
			require.NoError(t, getErr)

			snapshotOutput := &v1alpha1.Snapshot{}
			err = client.Get(context.Background(), types.NamespacedName{
				Namespace: configuration.Namespace,
				Name:      configuration.Status.SnapshotName,
			}, snapshotOutput)

			t.Log("check if target snapshot has been created and cache was called")
			require.NoError(t, err)

			t.Log("extracting the passed in data and checking if the configuration worked")
			args := cache.PushDataCallingArgumentsOnCall(0)
			assert.Equal(t, "sha-13092443426051895747", args.Name)
			assert.Equal(t, "1.0.0", args.Version)
			sourceFile := extractFileFromTarGz(t, io.NopCloser(bytes.NewBuffer([]byte(args.Content))), "configmap.yaml")
			configMap := corev1.ConfigMap{}
			assert.NoError(t, yaml.Unmarshal(sourceFile, &configMap))
			assert.Equal(t, map[string]string{"PODINFO_UI_MESSAGE": "this is a new message", "PODINFO_UI_COLOR": "bittersweet"}, configMap.Data)

			err = client.Get(context.Background(), types.NamespacedName{
				Namespace: configuration.Namespace,
				Name:      configuration.Name,
			}, configuration)
			require.NoError(t, err)

			assert.True(t, conditions.IsTrue(configuration, meta.ReadyCondition))

			close(recorder.Events)
			event := ""
			for e := range recorder.Events {
				if strings.Contains(e, "Reconciliation finished, next run in") {
					event = e

					break
				}
			}
			assert.Contains(t, event, "Reconciliation finished, next run in")
		})
	}
}

func TestPatchStrategicMergeWithGitRepositorySource(t *testing.T) {
	cv := DefaultComponent.DeepCopy()
	cv.Status.ComponentDescriptor = v1alpha1.Reference{
		Name:    "test-component",
		Version: "v0.0.1",
		ComponentDescriptorRef: meta.NamespacedObjectReference{
			Name:      cv.Name + "-descriptor",
			Namespace: cv.Namespace,
		},
	}
	conditions.MarkTrue(cv, meta.ReadyCondition, meta.SucceededReason, "test")

	cd := DefaultComponentDescriptor.DeepCopy()

	resource := DefaultResource.DeepCopy()

	name := "test-snapshot"
	identity := ocmmetav1.Identity{
		v1alpha1.ComponentNameKey:    cv.Status.ComponentDescriptor.ComponentDescriptorRef.Name,
		v1alpha1.ComponentVersionKey: cv.Status.ComponentDescriptor.Version,
		v1alpha1.ResourceNameKey:     resource.Spec.SourceRef.ResourceRef.Name,
		v1alpha1.ResourceVersionKey:  resource.Spec.SourceRef.ResourceRef.Version,
	}
	snapshot := &v1alpha1.Snapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cv.Namespace,
		},
		Spec: v1alpha1.SnapshotSpec{
			Identity: identity,
		},
	}

	resource.Status.SnapshotName = name
	conditions.MarkTrue(resource, meta.ReadyCondition, meta.SucceededReason, "test")

	source := v1alpha1.ObjectReference{
		NamespacedObjectKindReference: meta.NamespacedObjectKindReference{
			APIVersion: v1alpha1.GroupVersion.String(),
			Kind:       "Resource",
			Name:       "test-resource",
			Namespace:  snapshot.Namespace,
		},
	}

	path := "/file.tar.gz"
	server := ghttp.NewServer()
	server.RouteToHandler("GET", path, func(writer http.ResponseWriter, request *http.Request) {
		http.ServeFile(writer, request, "testdata/git-repo.tar.gz")
	})
	checksum := "87670827f3d1a10094e3226381c95168b6ce92344ac1a1c2345caaeb7cc6b7d8"
	gitRepo := createGitRepository("patch-repo", "default", server.URL()+path, checksum)

	configuration := DefaultConfiguration.DeepCopy()
	configuration.Spec.SourceRef = source
	configuration.Spec.ConfigRef = nil
	configuration.Status.SnapshotName = "configuration-snapshot"
	configuration.Spec.PatchStrategicMerge = &v1alpha1.PatchStrategicMerge{
		Source: v1alpha1.PatchStrategicMergeSource{
			SourceRef: meta.NamespacedObjectKindReference{
				Kind:      "GitRepository",
				Name:      gitRepo.Name,
				Namespace: gitRepo.Namespace,
			},
			Path: "sites/eu-west-1/deployment.yaml",
		},
		Target: v1alpha1.PatchStrategicMergeTarget{
			Path: "merge-target/merge-target.yaml",
		},
	}

	objs := []client.Object{cv, cd, resource, gitRepo, configuration}
	conditions.MarkTrue(snapshot, meta.ReadyCondition, meta.SucceededReason, "test")
	conditions.MarkTrue(gitRepo, meta.ReadyCondition, meta.SucceededReason, "test")
	objs = append(objs, snapshot)

	client := env.FakeKubeClient(WithObjects(objs...), WithAddToScheme(sourcev1.AddToScheme))
	dynClient := env.FakeDynamicKubeClient(WithObjects(objs...))
	cache := &cachefakes.FakeCache{}
	snapshotWriter := ocmsnapshot.NewOCIWriter(client, cache, env.scheme)
	fakeOcm := &fakes.MockFetcher{}
	recorder := record.NewFakeRecorder(32)
	content, err := os.Open(filepath.Join("testdata", "merge-target.tar.gz"))
	require.NoError(t, err)
	patchContent, err := os.Open(filepath.Join("testdata", "git-repo.tar.gz"))
	require.NoError(t, err)
	cache.FetchDataByDigestReturnsOnCall(0, content, nil)
	cache.FetchDataByDigestReturnsOnCall(1, patchContent, nil)
	fakeOcm.GetResourceReturnsOnCall(0, nil, nil)
	fakeOcm.GetResourceReturnsOnCall(1, nil, nil)

	cr := ConfigurationReconciler{
		Client:        client,
		DynamicClient: dynClient,
		Scheme:        env.scheme,
		EventRecorder: recorder,
		MutationReconciler: MutationReconcileLooper{
			Client:         client,
			DynamicClient:  dynClient,
			Scheme:         env.scheme,
			OCMClient:      fakeOcm,
			Cache:          cache,
			SnapshotWriter: snapshotWriter,
		},
	}

	t.Log("reconciling configuration")
	_, err = cr.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: configuration.Namespace,
			Name:      configuration.Name,
		},
	})
	require.NoError(t, err)

	getErr := client.Get(context.Background(), types.NamespacedName{
		Namespace: configuration.Namespace,
		Name:      configuration.Name,
	}, configuration)
	require.NoError(t, getErr)

	snapshotOutput := &v1alpha1.Snapshot{}
	err = client.Get(context.Background(), types.NamespacedName{
		Namespace: configuration.Namespace,
		Name:      configuration.Status.SnapshotName,
	}, snapshotOutput)

	t.Log("verifying that the strategic merge was performed")
	args := cache.PushDataCallingArgumentsOnCall(0)
	data := args.Content
	sourceFile := extractFileFromTarGz(t, io.NopCloser(bytes.NewBuffer([]byte(data))), "merge-target.yaml")
	deployment := appsv1.Deployment{}
	err = yaml.Unmarshal(sourceFile, &deployment)
	assert.NoError(t, err)
	assert.Equal(t, int32(2), *deployment.Spec.Replicas, "has correct number of replicas")
	assert.Equal(t, 2, len(deployment.Spec.Template.Spec.Containers), "has correct number of containers")
	assert.Equal(t, corev1.PullPolicy("Always"), deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy)

	close(recorder.Events)
	event := ""
	for e := range recorder.Events {
		if strings.Contains(e, "Reconciliation finished, next run in") {
			event = e

			break
		}
	}
	assert.Contains(t, event, "Reconciliation finished, next run in")
}

func TestConfigurationShouldReconcile(t *testing.T) {
	testcase := []struct {
		name             string
		errStr           string
		componentVersion func() *v1alpha1.ComponentVersion
		configuration    func(objs *[]client.Object) *v1alpha1.Configuration
	}{
		{
			name: "should not reconcile in case of matching generation and existing snapshot with ready state",
			componentVersion: func() *v1alpha1.ComponentVersion {
				cv := DefaultComponent.DeepCopy()
				cv.Status.ReconciledVersion = "v0.0.1"

				return cv
			},
			configuration: func(objs *[]client.Object) *v1alpha1.Configuration {
				configuration := DefaultConfiguration.DeepCopy()
				configuration.Status.LatestSourceVersion = "v0.0.1"
				configuration.Status.LatestConfigVersion = "v0.0.1"
				configuration.Spec.SourceRef.ResourceRef = &v1alpha1.ResourceReference{
					ElementMeta: v1alpha1.ElementMeta{
						Name: "name",
					},
				}
				snapshot := &v1alpha1.Snapshot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      configuration.Status.SnapshotName,
						Namespace: configuration.Namespace,
					},
					Spec:   v1alpha1.SnapshotSpec{},
					Status: v1alpha1.SnapshotStatus{},
				}
				conditions.MarkTrue(snapshot, meta.ReadyCondition, meta.SucceededReason, "Snapshot with name '%s' is ready", snapshot.Name)
				*objs = append(*objs, configuration, snapshot)

				return configuration
			},
		},
		{
			name: "should reconcile if snapshot is not ready",
			componentVersion: func() *v1alpha1.ComponentVersion {
				cv := DefaultComponent.DeepCopy()
				cv.Status.ReconciledVersion = "v0.0.1"

				return cv
			},
			configuration: func(objs *[]client.Object) *v1alpha1.Configuration {
				configuration := DefaultConfiguration.DeepCopy()
				configuration.Status.LatestSourceVersion = "v0.0.1"
				configuration.Status.LatestConfigVersion = "v0.0.1"
				configuration.Spec.SourceRef.ResourceRef = &v1alpha1.ResourceReference{
					ElementMeta: v1alpha1.ElementMeta{
						Name: "name",
					},
				}
				snapshot := &v1alpha1.Snapshot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      configuration.Status.SnapshotName,
						Namespace: configuration.Namespace,
					},
					Spec:   v1alpha1.SnapshotSpec{},
					Status: v1alpha1.SnapshotStatus{},
				}
				conditions.MarkFalse(snapshot, meta.ReadyCondition, meta.SucceededReason, "Snapshot with name '%s' is ready", snapshot.Name)

				*objs = append(*objs, configuration, snapshot)

				return configuration
			},
		},
		{
			name: "should not reconcile if source snapshot has the same digest",
			componentVersion: func() *v1alpha1.ComponentVersion {
				cv := DefaultComponent.DeepCopy()
				cv.Status.ReconciledVersion = "v0.0.1"

				return cv
			},
			configuration: func(objs *[]client.Object) *v1alpha1.Configuration {
				configuration := DefaultConfiguration.DeepCopy()
				configuration.Status.LatestSourceVersion = "last-reconciled-digest"
				configuration.Status.LatestConfigVersion = "v0.0.1"
				configuration.Spec.SourceRef = v1alpha1.ObjectReference{
					NamespacedObjectKindReference: meta.NamespacedObjectKindReference{
						Kind:      "Snapshot",
						Name:      "source-snapshot",
						Namespace: configuration.Namespace,
					},
				}
				snapshot := &v1alpha1.Snapshot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      configuration.Status.SnapshotName,
						Namespace: configuration.Namespace,
					},
					Spec: v1alpha1.SnapshotSpec{},
					Status: v1alpha1.SnapshotStatus{
						LastReconciledDigest: "last-reconciled-digest",
					},
				}
				conditions.MarkTrue(snapshot, meta.ReadyCondition, meta.SucceededReason, "Snapshot with name '%s' is ready", snapshot.Name)
				sourceSnapshot := &v1alpha1.Snapshot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "source-snapshot",
						Namespace: configuration.Namespace,
					},
					Status: v1alpha1.SnapshotStatus{
						LastReconciledDigest: "last-reconciled-digest",
						LastReconciledTag:    "latest",
					},
				}
				*objs = append(*objs, configuration, snapshot, sourceSnapshot)

				return configuration
			},
		},
		{
			name: "should not reconcile if there is no difference in config source",
			componentVersion: func() *v1alpha1.ComponentVersion {
				cv := DefaultComponent.DeepCopy()
				cv.Status.ReconciledVersion = "v0.0.1"

				return cv
			},
			configuration: func(objs *[]client.Object) *v1alpha1.Configuration {
				configuration := DefaultConfiguration.DeepCopy()
				configuration.Status.LatestSourceVersion = "v0.0.1"
				configuration.Status.LatestConfigVersion = "last-reconciled-digest"
				configuration.Spec.SourceRef = v1alpha1.ObjectReference{
					NamespacedObjectKindReference: meta.NamespacedObjectKindReference{
						Kind:      "ComponentVersion",
						Name:      "test-component",
						Namespace: configuration.Namespace,
					},
					ResourceRef: &v1alpha1.ResourceReference{
						ElementMeta: v1alpha1.ElementMeta{
							Name: "test",
						},
					},
				}
				configuration.Spec.ConfigRef = &v1alpha1.ObjectReference{
					NamespacedObjectKindReference: meta.NamespacedObjectKindReference{
						Kind:      "Snapshot",
						Name:      "config-snapshot",
						Namespace: configuration.Namespace,
					},
				}
				snapshot := &v1alpha1.Snapshot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      configuration.Status.SnapshotName,
						Namespace: configuration.Namespace,
					},
					Spec: v1alpha1.SnapshotSpec{},
					Status: v1alpha1.SnapshotStatus{
						LastReconciledDigest: "last-reconciled-digest",
					},
				}
				conditions.MarkTrue(snapshot, meta.ReadyCondition, meta.SucceededReason, "Snapshot with name '%s' is ready", snapshot.Name)
				configSnapshot := &v1alpha1.Snapshot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "config-snapshot",
						Namespace: configuration.Namespace,
					},
					Status: v1alpha1.SnapshotStatus{
						LastReconciledDigest: "last-reconciled-digest",
						LastReconciledTag:    "latest",
					},
				}
				*objs = append(*objs, configuration, configSnapshot, snapshot)

				return configuration
			},
		},
	}

	for i, tt := range testcase {
		t.Run(fmt.Sprintf("%d: %s", i, tt.name), func(t *testing.T) {
			var objs []client.Object
			configuration := tt.configuration(&objs)
			cv := tt.componentVersion()
			objs = append(objs, cv)
			client := env.FakeKubeClient(WithObjects(objs...), WithAddToScheme(sourcev1.AddToScheme))
			cache := &cachefakes.FakeCache{}
			fakeOcm := &fakes.MockFetcher{}
			dynClient := env.FakeDynamicKubeClient(WithObjects(objs...))

			cr := ConfigurationReconciler{
				DynamicClient: dynClient,
				Client:        client,
				Scheme:        env.scheme,
				OCMClient:     fakeOcm,
				EventRecorder: record.NewFakeRecorder(32),
				Cache:         cache,
			}

			result, err := cr.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: configuration.Namespace,
					Name:      configuration.Name,
				},
			})

			if tt.errStr == "" {
				require.NoError(t, err)
				assert.Equal(t, ctrl.Result{RequeueAfter: configuration.GetRequeueAfter()}, result)
				assert.True(t, cache.FetchDataByDigestWasNotCalled())
				assert.True(t, cache.PushDataWasNotCalled())
				assert.True(t, fakeOcm.GetResourceWasNotCalled())
			} else {
				assert.EqualError(t, err, tt.errStr)
			}
		})
	}
}

func TestPatchStrategicMergeWithResourceSource(t *testing.T) {
	testcases := []struct {
		name     string
		fileName string
	}{
		{
			name:     "should patch with gzip header",
			fileName: "git-repo.tar.gz",
		},
		{
			name:     "should patch with plain tar header",
			fileName: "git-repo.tar",
		},
	}

	for i, tt := range testcases {
		t.Run(fmt.Sprintf("%d: %s", i, tt.name), func(t *testing.T) {
			cv := DefaultComponent.DeepCopy()
			cv.Status.ComponentDescriptor = v1alpha1.Reference{
				Name:    "test-component",
				Version: "v0.0.1",
				ComponentDescriptorRef: meta.NamespacedObjectReference{
					Name:      cv.Name + "-descriptor",
					Namespace: cv.Namespace,
				},
			}
			conditions.MarkTrue(cv, meta.ReadyCondition, meta.SucceededReason, "test")

			cd := DefaultComponentDescriptor.DeepCopy()

			resource := DefaultResource.DeepCopy()
			patchResourceSource := DefaultResource.DeepCopy()
			patchResourceSource.Name = "patch-test-resource"

			name := "test-snapshot"
			identity := ocmmetav1.Identity{
				v1alpha1.ComponentNameKey:    cv.Status.ComponentDescriptor.ComponentDescriptorRef.Name,
				v1alpha1.ComponentVersionKey: cv.Status.ComponentDescriptor.Version,
				v1alpha1.ResourceNameKey:     resource.Spec.SourceRef.ResourceRef.Name,
				v1alpha1.ResourceVersionKey:  resource.Spec.SourceRef.ResourceRef.Version,
			}
			snapshot := &v1alpha1.Snapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: cv.Namespace,
				},
				Spec: v1alpha1.SnapshotSpec{
					Identity: identity,
				},
			}
			patchSnapshotSource := &v1alpha1.Snapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "patch-" + name,
					Namespace: cv.Namespace,
				},
				Spec: v1alpha1.SnapshotSpec{
					Identity: ocmmetav1.Identity{
						v1alpha1.ComponentNameKey:    cv.Status.ComponentDescriptor.ComponentDescriptorRef.Name,
						v1alpha1.ComponentVersionKey: cv.Status.ComponentDescriptor.Version,
						v1alpha1.ResourceNameKey:     "patch-snapshot-source",
						v1alpha1.ResourceVersionKey:  resource.Spec.SourceRef.ResourceRef.Version,
					},
				},
			}

			patchResourceSource.Status.SnapshotName = patchSnapshotSource.Name
			resource.Status.SnapshotName = name
			conditions.MarkTrue(resource, meta.ReadyCondition, meta.SucceededReason, "test")
			conditions.MarkTrue(patchResourceSource, meta.ReadyCondition, meta.SucceededReason, "test")

			source := v1alpha1.ObjectReference{
				NamespacedObjectKindReference: meta.NamespacedObjectKindReference{
					APIVersion: v1alpha1.GroupVersion.String(),
					Kind:       "Resource",
					Name:       "test-resource",
					Namespace:  snapshot.Namespace,
				},
			}

			configuration := DefaultConfiguration.DeepCopy()
			configuration.Spec.SourceRef = source
			configuration.Spec.ConfigRef = nil
			configuration.Status.SnapshotName = "configuration-snapshot"
			configuration.Spec.PatchStrategicMerge = &v1alpha1.PatchStrategicMerge{
				Source: v1alpha1.PatchStrategicMergeSource{
					SourceRef: meta.NamespacedObjectKindReference{
						APIVersion: v1alpha1.GroupVersion.String(),
						Kind:       "Resource",
						Name:       patchResourceSource.Name,
						Namespace:  patchResourceSource.Namespace,
					},
					Path: "sites/eu-west-1/deployment.yaml",
				},
				Target: v1alpha1.PatchStrategicMergeTarget{
					Path: "merge-target/merge-target.yaml",
				},
			}

			objs := []client.Object{cv, cd, resource, patchResourceSource, configuration}
			conditions.MarkTrue(snapshot, meta.ReadyCondition, meta.SucceededReason, "test")
			conditions.MarkTrue(patchSnapshotSource, meta.ReadyCondition, meta.SucceededReason, "test")
			objs = append(objs, snapshot, patchSnapshotSource)

			client := env.FakeKubeClient(WithObjects(objs...), WithAddToScheme(sourcev1.AddToScheme))
			dynClient := env.FakeDynamicKubeClient(WithObjects(objs...))
			cache := &cachefakes.FakeCache{}
			snapshotWriter := ocmsnapshot.NewOCIWriter(client, cache, env.scheme)
			fakeOcm := &fakes.MockFetcher{}
			recorder := record.NewFakeRecorder(32)
			content, err := os.Open(filepath.Join("testdata", "merge-target.tar.gz"))
			require.NoError(t, err)
			patchContent, err := os.Open(filepath.Join("testdata", tt.fileName))
			require.NoError(t, err)
			cache.FetchDataByDigestReturnsOnCall(0, content, nil)
			cache.FetchDataByDigestReturnsOnCall(1, patchContent, nil)
			fakeOcm.GetResourceReturnsOnCall(0, nil, nil)
			fakeOcm.GetResourceReturnsOnCall(1, nil, nil)

			cr := ConfigurationReconciler{
				Client:        client,
				DynamicClient: dynClient,
				Scheme:        env.scheme,
				EventRecorder: recorder,
				MutationReconciler: MutationReconcileLooper{
					Client:         client,
					DynamicClient:  dynClient,
					Scheme:         env.scheme,
					OCMClient:      fakeOcm,
					Cache:          cache,
					SnapshotWriter: snapshotWriter,
				},
			}

			t.Log("reconciling configuration")
			_, err = cr.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: configuration.Namespace,
					Name:      configuration.Name,
				},
			})
			require.NoError(t, err)

			getErr := client.Get(context.Background(), types.NamespacedName{
				Namespace: configuration.Namespace,
				Name:      configuration.Name,
			}, configuration)
			require.NoError(t, getErr)

			snapshotOutput := &v1alpha1.Snapshot{}
			err = client.Get(context.Background(), types.NamespacedName{
				Namespace: configuration.Namespace,
				Name:      configuration.Status.SnapshotName,
			}, snapshotOutput)

			t.Log("verifying that the strategic merge was performed")
			args := cache.PushDataCallingArgumentsOnCall(0)
			data := args.Content
			sourceFile := extractFileFromTarGz(t, io.NopCloser(bytes.NewBuffer([]byte(data))), "merge-target.yaml")
			deployment := appsv1.Deployment{}
			err = yaml.Unmarshal(sourceFile, &deployment)
			assert.NoError(t, err)
			assert.Equal(t, int32(2), *deployment.Spec.Replicas, "has correct number of replicas")
			assert.Equal(t, 2, len(deployment.Spec.Template.Spec.Containers), "has correct number of containers")
			assert.Equal(t, corev1.PullPolicy("Always"), deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy)

			close(recorder.Events)
			event := ""
			for e := range recorder.Events {
				if strings.Contains(e, "Reconciliation finished, next run in") {
					event = e

					break
				}
			}
			assert.Contains(t, event, "Reconciliation finished, next run in")
		})
	}
}

func createGitRepository(name, namespace, artifactURL, checksum string) *sourcev1.GitRepository {
	updatedTime := time.Now()
	return &sourcev1.GitRepository{
		TypeMeta: metav1.TypeMeta{
			Kind:       "GitRepository",
			APIVersion: sourcev1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: sourcev1.GitRepositorySpec{
			URL: "https://github.com/" + namespace + "/" + name,
			Reference: &sourcev1.GitRepositoryRef{
				Branch: "master",
			},
			Interval: metav1.Duration{Duration: time.Second * 30},
		},
		Status: sourcev1.GitRepositoryStatus{
			ObservedGeneration: int64(1),
			Conditions: []metav1.Condition{
				{
					Type:               "Ready",
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.Time{Time: updatedTime},
					Reason:             "GitOperationSucceed",
					Message:            "Fetched revision: master/b8e362c206e3d0cbb7ed22ced771a0056455a2fb",
				},
			},
			Artifact: &sourcev1.Artifact{
				Path:           "gitrepository/flux-system/test-tf-controller/b8e362c206e3d0cbb7ed22ced771a0056455a2fb.tar.gz",
				URL:            artifactURL,
				Revision:       "master/b8e362c206e3d0cbb7ed22ced771a0056455a2fb",
				Digest:         checksum,
				LastUpdateTime: metav1.Time{Time: updatedTime},
			},
		},
	}
}

func extractFileFromTarGz(t *testing.T, data io.Reader, filename string) []byte {
	t.Helper()

	tarReader := tar.NewReader(data)

	result := &bytes.Buffer{}

	for {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}

		assert.NoError(t, err)

		if filepath.Base(header.Name) == filename {
			_, err := io.Copy(result, tarReader)
			assert.NoError(t, err)
			break
		}
	}

	return result.Bytes()
}
