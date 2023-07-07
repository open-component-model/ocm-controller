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
	ocmsnapshot "github.com/open-component-model/ocm-controller/pkg/snapshot"
	ocmmetav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/ocm.software/v3alpha1"
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
	source              func(snapshot *v1alpha1.Snapshot) v1alpha1.ObjectReference
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
				cv.Status.ReconciledVersion = "v0.0.1"
				return cv
			},
			componentDescriptor: func(owner client.Object) *v1alpha1.ComponentDescriptor {
				cd := DefaultComponentDescriptor.DeepCopy()
				err := controllerutil.SetOwnerReference(owner, cd, env.scheme)
				require.NoError(t, err)
				return cd
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
				content, err := os.Open(filepath.Join("testdata", "localization-deploy.tar"))
				require.NoError(t, err)
				fakeCache.FetchDataByDigestReturns(content, nil)
				fakeOcm.GetResourceReturnsOnCall(0, io.NopCloser(bytes.NewBuffer(localizationConfigData)), nil)
				cmp := getMockComponent(t, DefaultComponent)
				fakeOcm.GetComponentVersionReturnsForName(cmp.GetName(), cmp, nil)
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
			source: func(snapshot *v1alpha1.Snapshot) v1alpha1.ObjectReference {
				return v1alpha1.ObjectReference{
					NamespacedObjectKindReference: meta.NamespacedObjectKindReference{
						APIVersion: v1alpha1.GroupVersion.String(),
						Kind:       "ComponentVersion",
						Name:       DefaultComponent.Name,
						Namespace:  DefaultComponent.Namespace,
					},
					ResourceRef: &v1alpha1.ResourceReference{
						ElementMeta: v3alpha1.ElementMeta{
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
				fakeOcm.GetResourceReturnsOnCall(1, io.NopCloser(bytes.NewBuffer(localizationConfigData)), nil)
				cmp := getMockComponent(t, DefaultComponent)
				fakeOcm.GetComponentVersionReturnsForName(cmp.GetName(), cmp, nil)
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
				fakeCache.FetchDataByDigestReturns(nil, errors.New("boo"))
				cmp := getMockComponent(t, DefaultComponent)
				fakeOcm.GetComponentVersionReturnsForName(cmp.GetName(), cmp, nil)
			},
		},
		{
			name:        "expect error when get resource fails without snapshots",
			expectError: "failed to fetch resource from component version: boo",
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
			source: func(snapshot *v1alpha1.Snapshot) v1alpha1.ObjectReference {
				return v1alpha1.ObjectReference{
					NamespacedObjectKindReference: meta.NamespacedObjectKindReference{
						APIVersion: v1alpha1.GroupVersion.String(),
						Kind:       "ComponentVersion",
						Name:       DefaultComponent.Name,
						Namespace:  DefaultComponent.Namespace,
					},
					ResourceRef: &v1alpha1.ResourceReference{
						ElementMeta: v3alpha1.ElementMeta{
							Name:    "some-resource",
							Version: "1.0.0",
						},
					},
				}
			},
			mock: func(fakeCache *cachefakes.FakeCache, fakeOcm *fakes.MockFetcher) {
				fakeOcm.GetResourceReturns(nil, "", errors.New("boo"))
				cmp := getMockComponent(t, DefaultComponent)
				fakeOcm.GetComponentVersionReturnsForName(cmp.GetName(), cmp, nil)
			},
		},
		{
			name:        "get resource fails during config data fetch",
			expectError: "failed to fetch resource from component version: boo",
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
			source: func(snapshot *v1alpha1.Snapshot) v1alpha1.ObjectReference {
				return v1alpha1.ObjectReference{
					NamespacedObjectKindReference: meta.NamespacedObjectKindReference{
						APIVersion: v1alpha1.GroupVersion.String(),
						Kind:       "ComponentVersion",
						Name:       DefaultComponent.Name,
						Namespace:  DefaultComponent.Namespace,
					},
					ResourceRef: &v1alpha1.ResourceReference{
						ElementMeta: v3alpha1.ElementMeta{
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
				fakeOcm.GetResourceReturnsOnCall(1, nil, errors.New("boo"))
				cmp := getMockComponent(t, DefaultComponent)
				fakeOcm.GetComponentVersionReturnsForName(cmp.GetName(), cmp, nil)
			},
		},
		{
			name:        "GetImageReference fails",
			expectError: "failed to parse access reference: cannot determine access spec type",
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
			source: func(snapshot *v1alpha1.Snapshot) v1alpha1.ObjectReference {
				return v1alpha1.ObjectReference{
					NamespacedObjectKindReference: meta.NamespacedObjectKindReference{
						APIVersion: v1alpha1.GroupVersion.String(),
						Kind:       "ComponentVersion",
						Name:       DefaultComponent.Name,
						Namespace:  DefaultComponent.Namespace,
					},
					ResourceRef: &v1alpha1.ResourceReference{
						ElementMeta: v3alpha1.ElementMeta{
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
				fakeOcm.GetResourceReturnsOnCall(1, io.NopCloser(bytes.NewBuffer(localizationConfigData)), nil)
				cmp := getMockComponent(t, DefaultComponent, setAccessType("unknown"))
				fakeOcm.GetComponentVersionReturnsForName(cmp.GetName(), cmp, nil)
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
				err := controllerutil.SetOwnerReference(owner, cd, env.scheme)
				require.NoError(t, err)
				return cd
			},
			snapshot: func(cv *v1alpha1.ComponentVersion, resource *v1alpha1.Resource) *v1alpha1.Snapshot {
				// do nothing
				return nil
			},
			source: func(snapshot *v1alpha1.Snapshot) v1alpha1.ObjectReference {
				return v1alpha1.ObjectReference{
					NamespacedObjectKindReference: meta.NamespacedObjectKindReference{
						APIVersion: v1alpha1.GroupVersion.String(),
						Kind:       "ComponentVersion",
						Name:       DefaultComponent.Name,
						Namespace:  DefaultComponent.Namespace,
					},
					ResourceRef: &v1alpha1.ResourceReference{
						ElementMeta: v3alpha1.ElementMeta{
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
				fakeOcm.GetResourceReturnsOnCall(1, io.NopCloser(bytes.NewBuffer(localizationConfigData)), nil)
				cmp := getMockComponent(t, DefaultComponent, setAccessRef("invalid:@:1.0.0"))
				fakeOcm.GetComponentVersionReturnsForName(cmp.GetName(), cmp, nil)
			},
		},
		{
			name:        "the returned content is not a tar file",
			expectError: "expected tarred directory content for configuration/localization resources, got plain text",
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
			source: func(snapshot *v1alpha1.Snapshot) v1alpha1.ObjectReference {
				return v1alpha1.ObjectReference{
					NamespacedObjectKindReference: meta.NamespacedObjectKindReference{
						APIVersion: v1alpha1.GroupVersion.String(),
						Kind:       "ComponentVersion",
						Name:       DefaultComponent.Name,
						Namespace:  DefaultComponent.Namespace,
					},
					ResourceRef: &v1alpha1.ResourceReference{
						ElementMeta: v3alpha1.ElementMeta{
							Name:    "some-resource",
							Version: "1.0.0",
						},
					},
				}
			},
			mock: func(fakeCache *cachefakes.FakeCache, fakeOcm *fakes.MockFetcher) {
				fakeOcm.GetResourceReturnsOnCall(0, io.NopCloser(bytes.NewBuffer([]byte("I am not a tar file"))), nil)
				fakeOcm.GetResourceReturnsOnCall(1, io.NopCloser(bytes.NewBuffer(localizationConfigData)), nil)
				cmp := getMockComponent(t, DefaultComponent)
				fakeOcm.GetComponentVersionReturnsForName(cmp.GetName(), cmp, nil)
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
			source: func(snapshot *v1alpha1.Snapshot) v1alpha1.ObjectReference {
				return v1alpha1.ObjectReference{
					NamespacedObjectKindReference: meta.NamespacedObjectKindReference{
						APIVersion: v1alpha1.GroupVersion.String(),
						Kind:       "ComponentVersion",
						Name:       DefaultComponent.Name,
						Namespace:  DefaultComponent.Namespace,
					},
					ResourceRef: &v1alpha1.ResourceReference{
						ElementMeta: v3alpha1.ElementMeta{
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
				cmp := getMockComponent(t, DefaultComponent)
				fakeOcm.GetComponentVersionReturnsForName(cmp.GetName(), cmp, nil)
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
			source: func(snapshot *v1alpha1.Snapshot) v1alpha1.ObjectReference {
				return v1alpha1.ObjectReference{
					NamespacedObjectKindReference: meta.NamespacedObjectKindReference{
						APIVersion: v1alpha1.GroupVersion.String(),
						Kind:       "ComponentVersion",
						Name:       DefaultComponent.Name,
						Namespace:  DefaultComponent.Namespace,
					},
					ResourceRef: &v1alpha1.ResourceReference{
						ElementMeta: v3alpha1.ElementMeta{
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
				cmp := getMockComponent(t, DefaultComponent)
				fakeOcm.GetComponentVersionReturnsForName(cmp.GetName(), cmp, nil)
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
			source: func(snapshot *v1alpha1.Snapshot) v1alpha1.ObjectReference {
				return v1alpha1.ObjectReference{
					NamespacedObjectKindReference: meta.NamespacedObjectKindReference{
						APIVersion: v1alpha1.GroupVersion.String(),
						Kind:       "ComponentVersion",
						Name:       DefaultComponent.Name,
						Namespace:  DefaultComponent.Namespace,
					},
					ResourceRef: &v1alpha1.ResourceReference{
						ElementMeta: v3alpha1.ElementMeta{
							Name:    "some-resource",
							Version: "1.0.0",
						},
					},
				}
			},
			mock: func(fakeCache *cachefakes.FakeCache, fakeOcm *fakes.MockFetcher) {
				content, err := os.Open(filepath.Join("testdata", "localization-deploy.tar"))
				require.NoError(t, err)
				fakeCache.PushDataReturns("", errors.New("boo"))
				fakeOcm.GetResourceReturnsOnCall(0, content, nil)
				fakeOcm.GetResourceReturnsOnCall(1, io.NopCloser(bytes.NewBuffer(localizationConfigData)), nil)
				cmp := getMockComponent(t, DefaultComponent)
				fakeOcm.GetComponentVersionReturnsForName(cmp.GetName(), cmp, nil)
			},
		},
	}
	for i, tt := range testCases {
		t.Run(fmt.Sprintf("%d: %s", i, tt.name), func(t *testing.T) {
			cv := tt.componentVersion()
			conditions.MarkTrue(cv, meta.ReadyCondition, meta.SucceededReason, "test")

			cd := tt.componentDescriptor(cv)

			resource := DefaultResource.DeepCopy()

			snapshot := tt.snapshot(cv, resource)

			source := tt.source(snapshot)

			localization := DefaultLocalization.DeepCopy()
			localization.Spec.SourceRef = source
			localization.Status.SnapshotName = "localization-snapshot"

			objs := []client.Object{cv, resource, cd, localization}

			if snapshot != nil {
				conditions.MarkTrue(snapshot, meta.ReadyCondition, meta.SucceededReason, "test")
				objs = append(objs, snapshot)
			}

			client := env.FakeKubeClient(WithObjects(objs...))
			dynClient := env.FakeDynamicKubeClient(WithObjects(objs...))
			cache := &cachefakes.FakeCache{}
			fakeOcm := &fakes.MockFetcher{}
			recorder := record.NewFakeRecorder(32)
			snapshotWriter := ocmsnapshot.NewOCIWriter(client, cache, env.scheme)
			tt.mock(cache, fakeOcm)

			lr := LocalizationReconciler{
				Client:        client,
				DynamicClient: dynClient,
				Scheme:        env.scheme,
				OCMClient:     fakeOcm,
				EventRecorder: recorder,
				Cache:         cache,
				MutationReconciler: MutationReconcileLooper{
					Client:         client,
					DynamicClient:  dynClient,
					Scheme:         env.scheme,
					OCMClient:      fakeOcm,
					Cache:          cache,
					SnapshotWriter: snapshotWriter,
				},
			}

			t.Log("reconciling localization")
			_, err := lr.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: localization.Namespace,
					Name:      localization.Name,
				},
			})

			getErr := client.Get(context.Background(), types.NamespacedName{
				Namespace: localization.Namespace,
				Name:      localization.Name,
			}, localization)
			require.NoError(t, getErr)

			if tt.expectError != "" {
				require.ErrorContains(t, err, tt.expectError)
				assert.True(t, conditions.IsFalse(localization, meta.ReadyCondition))
				return
			}
			require.NoError(t, err)
			t.Log("check if target snapshot has been created and cache was called")
			snapshotOutput := &v1alpha1.Snapshot{}
			err = client.Get(context.Background(), types.NamespacedName{
				Namespace: localization.Namespace,
				Name:      localization.Status.SnapshotName,
			}, snapshotOutput)
			require.NoError(t, err)
			args := cache.PushDataCallingArgumentsOnCall(0)
			assert.Equal(t, "sha-18322151501422808564", args.Name)
			assert.Equal(t, "999", args.Version)

			t.Log("extracting the passed in data and checking if the localization worked")
			require.NoError(t, err)
			assert.Contains(
				t,
				args.Content,
				"image: ghcr.io/mandelsoft/cnudie/component-descriptors/github.com/vasu1124/introspect@sha256:7f0168496f273c1e2095703a050128114d339c580b0906cd124a93b66ae471e2",
				"the image should have been altered during localization",
			)

			assert.Contains(
				t,
				args.Content,
				"registry: ghcr.io",
				"the registry should have been altered during localization",
			)

			assert.Contains(
				t,
				args.Content,
				"repository: mandelsoft/cnudie/component-descriptors/github.com/vasu1124/introspect",
				"the repository should have been altered during localization",
			)

			assert.Contains(
				t,
				args.Content,
				"tag: sha256:7f0168496f273c1e2095703a050128114d339c580b0906cd124a93b66ae471e2",
				"the reference should have been altered during localization",
			)

			assert.Contains(
				t,
				args.Content,
				"version: v0.0.1",
				"the labels should have been added via the localization mapping",
			)

			assert.Contains(
				t,
				args.Content,
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

		})
	}
}

// TODO: rewrite these so that they test the predicate functions
func XTestLocalizationShouldReconcile(t *testing.T) {
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
				localization.Status.LatestSourceVersion = "v0.0.1"
				localization.Status.LatestConfigVersion = "v0.0.1"
				localization.Spec.SourceRef.ResourceRef = &v1alpha1.ResourceReference{
					ElementMeta: v3alpha1.ElementMeta{
						Name: "name",
					},
				}
				snapshot := &v1alpha1.Snapshot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      localization.Status.SnapshotName,
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
				localization.Status.LatestSourceVersion = "v0.0.1"
				localization.Status.LatestConfigVersion = "v0.0.1"
				localization.Spec.SourceRef.ResourceRef = &v1alpha1.ResourceReference{
					ElementMeta: v3alpha1.ElementMeta{
						Name: "name",
					},
				}
				snapshot := &v1alpha1.Snapshot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      localization.Status.SnapshotName,
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
				localization.Status.LatestSourceVersion = "v0.0.1"
				localization.Status.LatestConfigVersion = "v0.0.1"
				localization.Spec.SourceRef.ResourceRef = &v1alpha1.ResourceReference{
					ElementMeta: v3alpha1.ElementMeta{
						Name: "name",
					},
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
				localization.Status.LatestSourceVersion = "not-last-reconciled-digest"
				localization.Status.LatestConfigVersion = "v0.0.1"
				localization.Spec.SourceRef.ResourceRef = &v1alpha1.ResourceReference{
					ElementMeta: v3alpha1.ElementMeta{
						Name: "name",
					},
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
				localization.Status.LatestSourceVersion = "last-reconciled-digest"
				localization.Status.LatestConfigVersion = "v0.0.1"
				localization.Spec.SourceRef = v1alpha1.ObjectReference{
					NamespacedObjectKindReference: meta.NamespacedObjectKindReference{
						Kind:      "Snapshot",
						Name:      "source-snapshot",
						Namespace: localization.Namespace,
					},
				}
				snapshot := &v1alpha1.Snapshot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      localization.Status.SnapshotName,
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
				localization.Status.LatestSourceVersion = "v0.0.1"
				localization.Status.LatestConfigVersion = "last-reconciled-digest"
				localization.Spec.SourceRef = v1alpha1.ObjectReference{
					NamespacedObjectKindReference: meta.NamespacedObjectKindReference{
						Kind:      "Snapshot",
						Name:      "source-snapshot",
						Namespace: localization.Namespace,
					},
				}
				localization.Spec.ConfigRef = &v1alpha1.ObjectReference{
					NamespacedObjectKindReference: meta.NamespacedObjectKindReference{
						Kind:      "Snapshot",
						Name:      "config-snapshot",
						Namespace: localization.Namespace,
					},
				}
				snapshot := &v1alpha1.Snapshot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      localization.Status.SnapshotName,
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
				localization.Status.LatestSourceVersion = "v0.0.1"
				localization.Status.LatestConfigVersion = "last-reconciled-digest"
				localization.Spec.SourceRef.ResourceRef = &v1alpha1.ResourceReference{
					ElementMeta: v3alpha1.ElementMeta{
						Name: "test",
					},
				}
				localization.Spec.ConfigRef = &v1alpha1.ObjectReference{
					NamespacedObjectKindReference: meta.NamespacedObjectKindReference{
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
				localization.Status.LatestSourceVersion = "v0.0.1"
				localization.Status.LatestConfigVersion = "last-reconciled-digest"
				localization.Spec.SourceRef = v1alpha1.ObjectReference{
					NamespacedObjectKindReference: meta.NamespacedObjectKindReference{
						Kind:      "ComponentVersion",
						Name:      "test-component",
						Namespace: localization.Namespace,
					},
					ResourceRef: &v1alpha1.ResourceReference{
						ElementMeta: v3alpha1.ElementMeta{
							Name: "test",
						},
					},
				}
				localization.Spec.PatchStrategicMerge = &v1alpha1.PatchStrategicMerge{
					Source: v1alpha1.PatchStrategicMergeSource{
						SourceRef: meta.NamespacedObjectKindReference{
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

			client := env.FakeKubeClient(WithObjects(objs...), WithAddToScheme(v1beta2.AddToScheme))
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
