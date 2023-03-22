// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/source-controller/api/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	cachefakes "github.com/open-component-model/ocm-controller/pkg/cache/fakes"
	"github.com/open-component-model/ocm-controller/pkg/ocm/fakes"
)

var localizationConfigData = []byte(`kind: ConfigData
metadata:
  name: test-config-data
  namespace: default
localization:
- file: deploy.yaml
  image: spec.template.spec.containers[0].image
  resource:
    name: introspect-image
- file: values.yaml
  registry: image.registry
  repository: image.repository
  tag: image.tag
  resource:
    name: introspect-image
- file: custom_resource.yaml
  mapping:
    path: metadata.labels
    transform: |-
        import "encoding/json"
        customLabels: {
            version: component.version
            env: "test"
        }
        out: json.Marshal(customLabels)
- file: custom_resource.yaml
  mapping:
    path: spec.values
    transform: |-
        import "encoding/json"
        values: [for x in component.resources {
          name: "\(x.name)-\(x.digest.hashAlgorithm)-\(x.version)"
          image: x.access.globalAccess.ref
        }]
        out: json.Marshal(values)
`)

type testCase struct {
	name                string
	mock                func(fakeCache *cachefakes.FakeCache, fakeOcm *fakes.MockFetcher)
	componentVersion    func() *v1alpha1.ComponentVersion
	componentDescriptor func(owner client.Object) *v1alpha1.ComponentDescriptor
	snapshot            func(cv *v1alpha1.ComponentVersion, resource *v1alpha1.Resource) *v1alpha1.Snapshot
	source              func(snapshot *v1alpha1.Snapshot) v1alpha1.Source
	expectError         string
}

func TestLocalizationReconciler(t *testing.T) {
	testCases := []testCase{
		{
			name: "with snapshot as a source",
			componentVersion: func() *v1alpha1.ComponentVersion {
				cv := DefaultComponent.DeepCopy()
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
			componentDescriptor: func(owner client.Object) *v1alpha1.ComponentDescriptor {
				cd := DefaultComponentDescriptor.DeepCopy()
				err := controllerutil.SetOwnerReference(owner, cd, env.scheme)
				require.NoError(t, err)
				return cd
			},
			source: func(snapshot *v1alpha1.Snapshot) v1alpha1.Source {
				return v1alpha1.Source{
					SourceRef: &meta.NamespacedObjectKindReference{
						APIVersion: v1alpha1.GroupVersion.String(),
						Kind:       "Snapshot",
						Name:       snapshot.Name,
						Namespace:  snapshot.Namespace,
					},
				}
			},
			snapshot: func(cv *v1alpha1.ComponentVersion, resource *v1alpha1.Resource) *v1alpha1.Snapshot {
				identity := v1alpha1.Identity{
					v1alpha1.ComponentNameKey:    cv.Spec.Component,
					v1alpha1.ComponentVersionKey: cv.Status.ReconciledVersion,
					v1alpha1.ResourceNameKey:     resource.Spec.Resource.Name,
					v1alpha1.ResourceVersionKey:  resource.Spec.Resource.Version,
				}
				sourceSnapshot := &v1alpha1.Snapshot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-snapshot",
						Namespace: cv.Namespace,
					},
					Spec: v1alpha1.SnapshotSpec{
						Identity: identity,
					},
				}
				return sourceSnapshot
			},
			mock: func(fakeCache *cachefakes.FakeCache, fakeOcm *fakes.MockFetcher) {
				content, err := os.Open(filepath.Join("testdata", "localization-deploy.tar"))
				require.NoError(t, err)
				fakeCache.FetchDataByDigestReturns(content, nil)
				fakeOcm.GetResourceReturns(io.NopCloser(bytes.NewBuffer(localizationConfigData)), "digest", nil)
			},
		},
		{
			name: "with resource as a source",
			componentVersion: func() *v1alpha1.ComponentVersion {
				cv := DefaultComponent.DeepCopy()
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
			componentDescriptor: func(owner client.Object) *v1alpha1.ComponentDescriptor {
				cd := DefaultComponentDescriptor.DeepCopy()
				err := controllerutil.SetOwnerReference(owner, cd, env.scheme)
				require.NoError(t, err)
				return cd
			},
			snapshot: func(cv *v1alpha1.ComponentVersion, resource *v1alpha1.Resource) *v1alpha1.Snapshot {
				// do nothing
				return nil
			},
			source: func(snapshot *v1alpha1.Snapshot) v1alpha1.Source {
				return v1alpha1.Source{
					ResourceRef: &v1alpha1.ResourceRef{
						Name:    "some-resource",
						Version: "1.0.0",
					},
				}
			},
			mock: func(fakeCache *cachefakes.FakeCache, fakeOcm *fakes.MockFetcher) {
				content, err := os.Open(filepath.Join("testdata", "localization-deploy.tar"))
				require.NoError(t, err)
				fakeOcm.GetResourceReturnsOnCall(0, content, nil)
				fakeOcm.GetResourceReturnsOnCall(1, io.NopCloser(bytes.NewBuffer(localizationConfigData)), nil)
			},
		},
		{
			name:        "expect error when neither source or resource ref is defined as a source",
			expectError: "either sourceRef or resourceRef should be defined, but both are empty",
			componentVersion: func() *v1alpha1.ComponentVersion {
				cv := DefaultComponent.DeepCopy()
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
			componentDescriptor: func(owner client.Object) *v1alpha1.ComponentDescriptor {
				cd := DefaultComponentDescriptor.DeepCopy()
				err := controllerutil.SetOwnerReference(owner, cd, env.scheme)
				require.NoError(t, err)
				return cd
			},
			snapshot: func(cv *v1alpha1.ComponentVersion, resource *v1alpha1.Resource) *v1alpha1.Snapshot {
				// do nothing
				return nil
			},
			source: func(snapshot *v1alpha1.Snapshot) v1alpha1.Source {
				// do nothing
				return v1alpha1.Source{}
			},
			mock: func(fakeCache *cachefakes.FakeCache, fakeOcm *fakes.MockFetcher) {
				content, err := os.Open(filepath.Join("testdata", "localization-deploy.tar"))
				require.NoError(t, err)
				fakeOcm.GetResourceReturns(content, "digest", nil)
			},
		},
		{
			name:        "expect error when get resource fails with snapshot",
			expectError: "failed to fetch resource data from snapshot: failed to fetch data: boo",
			componentVersion: func() *v1alpha1.ComponentVersion {
				cv := DefaultComponent.DeepCopy()
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
			componentDescriptor: func(owner client.Object) *v1alpha1.ComponentDescriptor {
				cd := DefaultComponentDescriptor.DeepCopy()
				err := controllerutil.SetOwnerReference(owner, cd, env.scheme)
				require.NoError(t, err)
				return cd
			},
			source: func(snapshot *v1alpha1.Snapshot) v1alpha1.Source {
				return v1alpha1.Source{
					SourceRef: &meta.NamespacedObjectKindReference{
						APIVersion: v1alpha1.GroupVersion.String(),
						Kind:       "Snapshot",
						Name:       snapshot.Name,
						Namespace:  snapshot.Namespace,
					},
				}
			},
			snapshot: func(cv *v1alpha1.ComponentVersion, resource *v1alpha1.Resource) *v1alpha1.Snapshot {
				identity := v1alpha1.Identity{
					v1alpha1.ComponentNameKey:    cv.Spec.Component,
					v1alpha1.ComponentVersionKey: cv.Status.ReconciledVersion,
					v1alpha1.ResourceNameKey:     resource.Spec.Resource.Name,
					v1alpha1.ResourceVersionKey:  resource.Spec.Resource.Version,
				}
				sourceSnapshot := &v1alpha1.Snapshot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-snapshot",
						Namespace: cv.Namespace,
					},
					Spec: v1alpha1.SnapshotSpec{
						Identity: identity,
					},
				}
				return sourceSnapshot
			},
			mock: func(fakeCache *cachefakes.FakeCache, fakeOcm *fakes.MockFetcher) {
				fakeCache.FetchDataByDigestReturns(nil, errors.New("boo"))
			},
		},
		{
			name:        "expect error when get resource fails without snapshots",
			expectError: "failed to fetch resource data from resource ref: failed to fetch resource from resource ref: boo",
			componentVersion: func() *v1alpha1.ComponentVersion {
				cv := DefaultComponent.DeepCopy()
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
			componentDescriptor: func(owner client.Object) *v1alpha1.ComponentDescriptor {
				cd := DefaultComponentDescriptor.DeepCopy()
				err := controllerutil.SetOwnerReference(owner, cd, env.scheme)
				require.NoError(t, err)
				return cd
			},
			snapshot: func(cv *v1alpha1.ComponentVersion, resource *v1alpha1.Resource) *v1alpha1.Snapshot {
				// do nothing
				return nil
			},
			source: func(snapshot *v1alpha1.Snapshot) v1alpha1.Source {
				return v1alpha1.Source{
					ResourceRef: &v1alpha1.ResourceRef{
						Name:    "some-resource",
						Version: "1.0.0",
					},
				}
			},
			mock: func(fakeCache *cachefakes.FakeCache, fakeOcm *fakes.MockFetcher) {
				fakeOcm.GetResourceReturns(nil, "", errors.New("boo"))
			},
		},
		{
			name:        "get resource fails during config data fetch",
			expectError: "failed to get resource: boo",
			componentVersion: func() *v1alpha1.ComponentVersion {
				cv := DefaultComponent.DeepCopy()
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
			componentDescriptor: func(owner client.Object) *v1alpha1.ComponentDescriptor {
				cd := DefaultComponentDescriptor.DeepCopy()
				err := controllerutil.SetOwnerReference(owner, cd, env.scheme)
				require.NoError(t, err)
				return cd
			},
			snapshot: func(cv *v1alpha1.ComponentVersion, resource *v1alpha1.Resource) *v1alpha1.Snapshot {
				// do nothing
				return nil
			},
			source: func(snapshot *v1alpha1.Snapshot) v1alpha1.Source {
				return v1alpha1.Source{
					ResourceRef: &v1alpha1.ResourceRef{
						Name:    "some-resource",
						Version: "1.0.0",
					},
				}
			},
			mock: func(fakeCache *cachefakes.FakeCache, fakeOcm *fakes.MockFetcher) {
				content, err := os.Open(filepath.Join("testdata", "localization-deploy.tar"))
				require.NoError(t, err)
				fakeOcm.GetResourceReturnsOnCall(0, content, nil)
				fakeOcm.GetResourceReturnsOnCall(1, nil, errors.New("boo"))
			},
		},
		{
			name:        "GetImageReference fails",
			expectError: "failed to get image access: failed to unmarshal access spec: json: Unmarshal(nil)",
			componentVersion: func() *v1alpha1.ComponentVersion {
				cv := DefaultComponent.DeepCopy()
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
			componentDescriptor: func(owner client.Object) *v1alpha1.ComponentDescriptor {
				cd := DefaultComponentDescriptor.DeepCopy()
				cd.Spec.Resources[0].Access.Object["type"] = "unknown"
				err := controllerutil.SetOwnerReference(owner, cd, env.scheme)
				require.NoError(t, err)
				return cd
			},
			snapshot: func(cv *v1alpha1.ComponentVersion, resource *v1alpha1.Resource) *v1alpha1.Snapshot {
				// do nothing
				return nil
			},
			source: func(snapshot *v1alpha1.Snapshot) v1alpha1.Source {
				return v1alpha1.Source{
					ResourceRef: &v1alpha1.ResourceRef{
						Name:    "some-resource",
						Version: "1.0.0",
					},
				}
			},
			mock: func(fakeCache *cachefakes.FakeCache, fakeOcm *fakes.MockFetcher) {
				content, err := os.Open(filepath.Join("testdata", "localization-deploy.tar"))
				require.NoError(t, err)
				fakeOcm.GetResourceReturnsOnCall(0, content, nil)
				fakeOcm.GetResourceReturnsOnCall(1, io.NopCloser(bytes.NewBuffer(localizationConfigData)), nil)
			},
		},
		{
			name:        "ParseReference fails",
			expectError: "failed to parse access reference: could not parse reference: invalid:@:1.0.0@sha256:7f0168496f273c1e2095703a050128114d339c580b0906cd124a93b66ae471e2",
			componentVersion: func() *v1alpha1.ComponentVersion {
				cv := DefaultComponent.DeepCopy()
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
			componentDescriptor: func(owner client.Object) *v1alpha1.ComponentDescriptor {
				cd := DefaultComponentDescriptor.DeepCopy()
				cd.Spec.Resources[0].Access.Object["globalAccess"].(map[string]any)["ref"] = "invalid:@"
				err := controllerutil.SetOwnerReference(owner, cd, env.scheme)
				require.NoError(t, err)
				return cd
			},
			snapshot: func(cv *v1alpha1.ComponentVersion, resource *v1alpha1.Resource) *v1alpha1.Snapshot {
				// do nothing
				return nil
			},
			source: func(snapshot *v1alpha1.Snapshot) v1alpha1.Source {
				return v1alpha1.Source{
					ResourceRef: &v1alpha1.ResourceRef{
						Name:    "some-resource",
						Version: "1.0.0",
					},
				}
			},
			mock: func(fakeCache *cachefakes.FakeCache, fakeOcm *fakes.MockFetcher) {
				content, err := os.Open(filepath.Join("testdata", "localization-deploy.tar"))
				require.NoError(t, err)
				fakeOcm.GetResourceReturnsOnCall(0, content, nil)
				fakeOcm.GetResourceReturnsOnCall(1, io.NopCloser(bytes.NewBuffer(localizationConfigData)), nil)
			},
		},
		{
			name:        "the returned content is not a tar file",
			expectError: "failed to reconcile mutation object: expected tarred directory content for configuration/localization resources, got plain text",
			componentVersion: func() *v1alpha1.ComponentVersion {
				cv := DefaultComponent.DeepCopy()
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
			componentDescriptor: func(owner client.Object) *v1alpha1.ComponentDescriptor {
				cd := DefaultComponentDescriptor.DeepCopy()
				err := controllerutil.SetOwnerReference(owner, cd, env.scheme)
				require.NoError(t, err)
				return cd
			},
			snapshot: func(cv *v1alpha1.ComponentVersion, resource *v1alpha1.Resource) *v1alpha1.Snapshot {
				// do nothing
				return nil
			},
			source: func(snapshot *v1alpha1.Snapshot) v1alpha1.Source {
				return v1alpha1.Source{
					ResourceRef: &v1alpha1.ResourceRef{
						Name:    "some-resource",
						Version: "1.0.0",
					},
				}
			},
			mock: func(fakeCache *cachefakes.FakeCache, fakeOcm *fakes.MockFetcher) {
				fakeOcm.GetResourceReturnsOnCall(0, io.NopCloser(bytes.NewBuffer([]byte("I am not a tar file"))), nil)
				fakeOcm.GetResourceReturnsOnCall(1, io.NopCloser(bytes.NewBuffer(localizationConfigData)), nil)
			},
		},
		{
			name:        "localization fails because the file does not exist",
			expectError: "no such file or directory",
			componentVersion: func() *v1alpha1.ComponentVersion {
				cv := DefaultComponent.DeepCopy()
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
			componentDescriptor: func(owner client.Object) *v1alpha1.ComponentDescriptor {
				cd := DefaultComponentDescriptor.DeepCopy()
				err := controllerutil.SetOwnerReference(owner, cd, env.scheme)
				require.NoError(t, err)
				return cd
			},
			snapshot: func(cv *v1alpha1.ComponentVersion, resource *v1alpha1.Resource) *v1alpha1.Snapshot {
				// do nothing
				return nil
			},
			source: func(snapshot *v1alpha1.Snapshot) v1alpha1.Source {
				return v1alpha1.Source{
					ResourceRef: &v1alpha1.ResourceRef{
						Name:    "some-resource",
						Version: "1.0.0",
					},
				}
			},
			mock: func(fakeCache *cachefakes.FakeCache, fakeOcm *fakes.MockFetcher) {
				content, err := os.Open(filepath.Join("testdata", "localization-deploy.tar"))
				require.NoError(t, err)
				fakeOcm.GetResourceReturnsOnCall(0, content, nil)
				testConfigData := []byte(`kind: ConfigData
metadata:
  name: test-config-data
  namespace: default
localization:
- file: idonotexist
  image: spec.template.spec.containers[0].image
  resource:
    name: introspect-image
`)
				fakeOcm.GetResourceReturnsOnCall(1, io.NopCloser(bytes.NewBuffer(testConfigData)), nil)
			},
		},
		{
			name:        "it fails to marshal the config data",
			expectError: "failed to unmarshal content",
			componentVersion: func() *v1alpha1.ComponentVersion {
				cv := DefaultComponent.DeepCopy()
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
			componentDescriptor: func(owner client.Object) *v1alpha1.ComponentDescriptor {
				cd := DefaultComponentDescriptor.DeepCopy()
				err := controllerutil.SetOwnerReference(owner, cd, env.scheme)
				require.NoError(t, err)
				return cd
			},
			snapshot: func(cv *v1alpha1.ComponentVersion, resource *v1alpha1.Resource) *v1alpha1.Snapshot {
				// do nothing
				return nil
			},
			source: func(snapshot *v1alpha1.Snapshot) v1alpha1.Source {
				return v1alpha1.Source{
					ResourceRef: &v1alpha1.ResourceRef{
						Name:    "some-resource",
						Version: "1.0.0",
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
			componentDescriptor: func(owner client.Object) *v1alpha1.ComponentDescriptor {
				cd := DefaultComponentDescriptor.DeepCopy()
				err := controllerutil.SetOwnerReference(owner, cd, env.scheme)
				require.NoError(t, err)
				return cd
			},
			snapshot: func(cv *v1alpha1.ComponentVersion, resource *v1alpha1.Resource) *v1alpha1.Snapshot {
				// do nothing
				return nil
			},
			source: func(snapshot *v1alpha1.Snapshot) v1alpha1.Source {
				return v1alpha1.Source{
					ResourceRef: &v1alpha1.ResourceRef{
						Name:    "some-resource",
						Version: "1.0.0",
					},
				}
			},
			mock: func(fakeCache *cachefakes.FakeCache, fakeOcm *fakes.MockFetcher) {
				content, err := os.Open(filepath.Join("testdata", "localization-deploy.tar"))
				require.NoError(t, err)
				fakeCache.PushDataReturns("", errors.New("boo"))
				fakeOcm.GetResourceReturnsOnCall(0, content, nil)
				fakeOcm.GetResourceReturnsOnCall(1, io.NopCloser(bytes.NewBuffer(localizationConfigData)), nil)
			},
		},
	}
	for i, tt := range testCases {
		t.Run(fmt.Sprintf("%d: %s", i, tt.name), func(t *testing.T) {
			cv := tt.componentVersion()
			resource := DefaultResource.DeepCopy()
			cd := tt.componentDescriptor(cv)
			snapshot := tt.snapshot(cv, resource)
			source := tt.source(snapshot)
			localization := DefaultLocalization.DeepCopy()
			localization.Spec.Source = source
			objs := []client.Object{cv, resource, cd, localization}
			if snapshot != nil {
				objs = append(objs, snapshot)
			}
			client := env.FakeKubeClient(WithObjets(objs...))
			cache := &cachefakes.FakeCache{}
			fakeOcm := &fakes.MockFetcher{}
			recorder := record.NewFakeRecorder(32)
			tt.mock(cache, fakeOcm)

			lr := LocalizationReconciler{
				Client:        client,
				Scheme:        env.scheme,
				OCMClient:     fakeOcm,
				EventRecorder: recorder,
				Cache:         cache,
			}

			_, err := lr.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: localization.Namespace,
					Name:      localization.Name,
				},
			})
			if tt.expectError != "" {
				require.ErrorContains(t, err, tt.expectError)
				err = client.Get(context.Background(), types.NamespacedName{
					Namespace: localization.Namespace,
					Name:      localization.Name,
				}, localization)
				require.NoError(t, err)

				assert.True(t, conditions.IsFalse(localization, meta.ReadyCondition))
			} else {
				require.NoError(t, err)
				t.Log("check if target snapshot has been created and cache was called")
				snapshotOutput := &v1alpha1.Snapshot{}
				err = client.Get(context.Background(), types.NamespacedName{
					Namespace: localization.Namespace,
					Name:      localization.Spec.SnapshotTemplate.Name,
				}, snapshotOutput)
				require.NoError(t, err)
				args := cache.PushDataCallingArgumentsOnCall(0)
				data, name, version := args[0], args[1], args[2]
				assert.Equal(t, "sha-1009814895297045910", name)
				assert.Equal(t, "999", version)

				t.Log("extracting the passed in data and checking if the localization worked")
				require.NoError(t, err)
				assert.Contains(
					t,
					data.(string),
					"image: ghcr.io/mandelsoft/cnudie/component-descriptors/github.com/vasu1124/introspect@sha256:7f0168496f273c1e2095703a050128114d339c580b0906cd124a93b66ae471e2",
					"the image should have been altered during localization",
				)

				assert.Contains(
					t,
					data.(string),
					"registry: ghcr.io",
					"the registry should have been altered during localization",
				)

				assert.Contains(
					t,
					data.(string),
					"repository: mandelsoft/cnudie/component-descriptors/github.com/vasu1124/introspect",
					"the repository should have been altered during localization",
				)

				assert.Contains(
					t,
					data.(string),
					"tag: sha256:7f0168496f273c1e2095703a050128114d339c580b0906cd124a93b66ae471e2",
					"the reference should have been altered during localization",
				)

				assert.Contains(
					t,
					data.(string),
					"version: v0.0.1",
					"the labels should have been added via the localization mapping",
				)

				assert.Contains(
					t,
					data.(string),
					"name: introspect-image-sha256-1.0.0",
					"the custome resource spec.values should have been updated via the localization mapping",
				)

				err = client.Get(context.Background(), types.NamespacedName{
					Namespace: localization.Namespace,
					Name:      localization.Name,
				}, localization)
				require.NoError(t, err)

				assert.True(t, conditions.IsTrue(localization, meta.ReadyCondition))

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
		})
	}
}

func TestLocalizationShouldReconcile(t *testing.T) {
	testcase := []struct {
		name             string
		errStr           string
		componentVersion func() *v1alpha1.ComponentVersion
		localization     func(objs *[]client.Object) *v1alpha1.Localization
	}{
		{
			name: "should not reconcile in case of matching generation and existing snapshot with ready state",
			componentVersion: func() *v1alpha1.ComponentVersion {
				cv := DefaultComponent.DeepCopy()
				cv.Status.ReconciledVersion = "v0.0.1"
				return cv
			},
			localization: func(objs *[]client.Object) *v1alpha1.Localization {
				localization := DefaultLocalization.DeepCopy()
				localization.Status.LastAppliedComponentVersion = "v0.0.1"
				localization.Spec.Source.ResourceRef = &v1alpha1.ResourceRef{
					Name: "name",
				}
				snapshot := &v1alpha1.Snapshot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      localization.Spec.SnapshotTemplate.Name,
						Namespace: localization.Namespace,
					},
					Spec:   v1alpha1.SnapshotSpec{},
					Status: v1alpha1.SnapshotStatus{},
				}
				conditions.MarkTrue(snapshot, meta.ReadyCondition, meta.SucceededReason, "Snapshot with name '%s' is ready", snapshot.Name)

				*objs = append(*objs, localization, snapshot)
				return localization
			},
		},
		{
			name:   "should reconcile if snapshot is not ready",
			errStr: "failed to reconcile mutation object: failed to fetch resource data from resource ref: failed to fetch resource from resource ref: unexpected number of calls; not enough return values have been configured; call count 0",
			componentVersion: func() *v1alpha1.ComponentVersion {
				cv := DefaultComponent.DeepCopy()
				cv.Status.ReconciledVersion = "v0.0.1"
				return cv
			},
			localization: func(objs *[]client.Object) *v1alpha1.Localization {
				localization := DefaultLocalization.DeepCopy()
				localization.Status.LastAppliedComponentVersion = "v0.0.1"
				localization.Spec.Source.ResourceRef = &v1alpha1.ResourceRef{
					Name: "name",
				}
				snapshot := &v1alpha1.Snapshot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      localization.Spec.SnapshotTemplate.Name,
						Namespace: localization.Namespace,
					},
					Spec:   v1alpha1.SnapshotSpec{},
					Status: v1alpha1.SnapshotStatus{},
				}
				conditions.MarkFalse(snapshot, meta.ReadyCondition, meta.SucceededReason, "Snapshot with name '%s' is ready", snapshot.Name)

				*objs = append(*objs, localization, snapshot)
				return localization
			},
		},
		{
			name:   "should reconcile if component version doesn't match",
			errStr: "failed to reconcile mutation object: failed to fetch resource data from resource ref: failed to fetch resource from resource ref: unexpected number of calls; not enough return values have been configured; call count 0",
			componentVersion: func() *v1alpha1.ComponentVersion {
				cv := DefaultComponent.DeepCopy()
				cv.Status.ReconciledVersion = "v0.0.2"
				return cv
			},
			localization: func(objs *[]client.Object) *v1alpha1.Localization {
				localization := DefaultLocalization.DeepCopy()
				localization.Status.LastAppliedComponentVersion = "v0.0.1"
				localization.Spec.Source.ResourceRef = &v1alpha1.ResourceRef{
					Name: "name",
				}

				*objs = append(*objs, localization)
				return localization
			},
		},
		{
			name:   "should reconcile if change was detected in source snapshot",
			errStr: "failed to reconcile mutation object: failed to fetch resource data from snapshot: failed to fetch data: unexpected number of calls; not enough return values have been configured; call count 0",
			componentVersion: func() *v1alpha1.ComponentVersion {
				cv := DefaultComponent.DeepCopy()
				cv.Status.ReconciledVersion = "v0.0.1"
				return cv
			},
			localization: func(objs *[]client.Object) *v1alpha1.Localization {
				localization := DefaultLocalization.DeepCopy()
				localization.Status.LastAppliedComponentVersion = "v0.0.1"
				localization.Status.LastAppliedSourceDigest = "not-last-reconciled-digest"
				localization.Spec.Source.SourceRef = &meta.NamespacedObjectKindReference{
					Kind:      "Snapshot",
					Name:      "source-snapshot",
					Namespace: localization.Namespace,
				}
				sourceSnapshot := &v1alpha1.Snapshot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "source-snapshot",
						Namespace: localization.Namespace,
					},
					Status: v1alpha1.SnapshotStatus{
						LastReconciledDigest: "last-reconciled-digest",
						LastReconciledTag:    "latest",
					},
				}
				*objs = append(*objs, localization, sourceSnapshot)
				return localization
			},
		},
		{
			name: "should not reconcile if source snapshot has the same digest",
			componentVersion: func() *v1alpha1.ComponentVersion {
				cv := DefaultComponent.DeepCopy()
				cv.Status.ReconciledVersion = "v0.0.1"
				return cv
			},
			localization: func(objs *[]client.Object) *v1alpha1.Localization {
				localization := DefaultLocalization.DeepCopy()
				localization.Status.LastAppliedComponentVersion = "v0.0.1"
				localization.Status.LastAppliedSourceDigest = "last-reconciled-digest"
				localization.Spec.Source.SourceRef = &meta.NamespacedObjectKindReference{
					Kind:      "Snapshot",
					Name:      "source-snapshot",
					Namespace: localization.Namespace,
				}
				snapshot := &v1alpha1.Snapshot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      localization.Spec.SnapshotTemplate.Name,
						Namespace: localization.Namespace,
					},
					Spec:   v1alpha1.SnapshotSpec{},
					Status: v1alpha1.SnapshotStatus{},
				}
				conditions.MarkTrue(snapshot, meta.ReadyCondition, meta.SucceededReason, "Snapshot with name '%s' is ready", snapshot.Name)
				sourceSnapshot := &v1alpha1.Snapshot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "source-snapshot",
						Namespace: localization.Namespace,
					},
					Status: v1alpha1.SnapshotStatus{
						LastReconciledDigest: "last-reconciled-digest",
						LastReconciledTag:    "latest",
					},
				}
				*objs = append(*objs, localization, sourceSnapshot, snapshot)
				return localization
			},
		},
		{
			name: "should not reconcile if there is no difference in config source",
			componentVersion: func() *v1alpha1.ComponentVersion {
				cv := DefaultComponent.DeepCopy()
				cv.Status.ReconciledVersion = "v0.0.1"
				return cv
			},
			localization: func(objs *[]client.Object) *v1alpha1.Localization {
				localization := DefaultLocalization.DeepCopy()
				localization.Status.LastAppliedComponentVersion = "v0.0.1"
				localization.Status.LastAppliedConfigSourceDigest = "last-reconciled-digest"
				localization.Spec.Source.ResourceRef = &v1alpha1.ResourceRef{
					Name: "test",
				}
				localization.Spec.ConfigRef.Resource = v1alpha1.Source{
					SourceRef: &meta.NamespacedObjectKindReference{
						Kind:      "Snapshot",
						Name:      "config-snapshot",
						Namespace: localization.Namespace,
					},
				}
				snapshot := &v1alpha1.Snapshot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      localization.Spec.SnapshotTemplate.Name,
						Namespace: localization.Namespace,
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
						Namespace: localization.Namespace,
					},
					Status: v1alpha1.SnapshotStatus{
						LastReconciledDigest: "last-reconciled-digest",
						LastReconciledTag:    "latest",
					},
				}
				*objs = append(*objs, localization, configSnapshot, snapshot)
				return localization
			},
		},
		{
			name:   "should reconcile if there is a difference in config source",
			errStr: "failed to reconcile mutation object: failed to fetch resource data from resource ref: failed to fetch resource from resource ref: unexpected number of calls; not enough return values have been configured; call count 0",
			componentVersion: func() *v1alpha1.ComponentVersion {
				cv := DefaultComponent.DeepCopy()
				cv.Status.ReconciledVersion = "v0.0.1"
				return cv
			},
			localization: func(objs *[]client.Object) *v1alpha1.Localization {
				localization := DefaultLocalization.DeepCopy()
				localization.Status.LastAppliedComponentVersion = "v0.0.1"
				localization.Status.LastAppliedConfigSourceDigest = "last-reconciled-digest"
				localization.Spec.Source.ResourceRef = &v1alpha1.ResourceRef{
					Name: "test",
				}
				localization.Spec.ConfigRef.Resource = v1alpha1.Source{
					SourceRef: &meta.NamespacedObjectKindReference{
						Kind:      "Snapshot",
						Name:      "config-snapshot",
						Namespace: localization.Namespace,
					},
				}
				configSnapshot := &v1alpha1.Snapshot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "config-snapshot",
						Namespace: localization.Namespace,
					},
					Status: v1alpha1.SnapshotStatus{
						LastReconciledDigest: "not-last-reconciled-digest",
						LastReconciledTag:    "latest",
					},
				}
				*objs = append(*objs, localization, configSnapshot)
				return localization
			},
		},
		{
			name:   "should reconcile if there is a difference in patch merge object",
			errStr: "failed to reconcile mutation object: failed to fetch resource data from resource ref: failed to fetch resource from resource ref: unexpected number of calls; not enough return values have been configured; call count 0",
			componentVersion: func() *v1alpha1.ComponentVersion {
				cv := DefaultComponent.DeepCopy()
				cv.Status.ReconciledVersion = "v0.0.1"
				return cv
			},
			localization: func(objs *[]client.Object) *v1alpha1.Localization {
				localization := DefaultLocalization.DeepCopy()
				localization.Status.LastAppliedComponentVersion = "v0.0.1"
				localization.Status.LastAppliedPatchMergeSourceDigest = "last-reconciled-digest"
				localization.Spec.Source.ResourceRef = &v1alpha1.ResourceRef{
					Name: "test",
				}
				localization.Spec.PatchStrategicMerge = &v1alpha1.PatchStrategicMerge{
					Source: v1alpha1.PatchStrategicMergeSource{
						SourceRef: v1alpha1.PatchStrategicMergeSourceRef{
							Kind:      "GitRepository",
							Name:      "git-test",
							Namespace: localization.Namespace,
						},
					},
				}
				gitrepo := createGitRepository("git-test", localization.Namespace, "url", "last-reconciled-digest")
				*objs = append(*objs, localization, gitrepo)
				return localization
			},
		},
	}

	for i, tt := range testcase {
		t.Run(fmt.Sprintf("%d: %s", i, tt.name), func(t *testing.T) {
			// We don't set a source because it shouldn't get that far.
			var objs []client.Object
			localization := tt.localization(&objs)
			cv := tt.componentVersion()

			objs = append(objs, cv)

			client := env.FakeKubeClient(WithObjets(objs...), WithAddToScheme(v1beta2.AddToScheme))
			cache := &cachefakes.FakeCache{}
			fakeOcm := &fakes.MockFetcher{}

			rr := LocalizationReconciler{
				Client:        client,
				Scheme:        env.scheme,
				OCMClient:     fakeOcm,
				EventRecorder: record.NewFakeRecorder(32),
				Cache:         cache,
			}

			result, err := rr.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: localization.Namespace,
					Name:      localization.Name,
				},
			})

			if tt.errStr == "" {
				require.NoError(t, err)
				assert.Equal(t, ctrl.Result{RequeueAfter: localization.GetRequeueAfter()}, result)
				assert.True(t, cache.FetchDataByDigestWasNotCalled())
				assert.True(t, cache.PushDataWasNotCalled())
				assert.True(t, fakeOcm.GetResourceWasNotCalled())
			} else {
				assert.EqualError(t, err, tt.errStr)
			}
		})
	}
}
