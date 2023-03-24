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
	"strings"
	"testing"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ocmmetav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/ocm.software/v3alpha1"
	ocmruntime "github.com/open-component-model/ocm/pkg/runtime"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	cachefakes "github.com/open-component-model/ocm-controller/pkg/cache/fakes"
	"github.com/open-component-model/ocm-controller/pkg/ocm/fakes"
)

func TestResourceReconciler(t *testing.T) {
	t.Log("setting up resource object")
	obj := DefaultResource.DeepCopy()
	// Tests that the component descriptor exists for root items.
	obj.Spec.Resource.ReferencePath = nil

	t.Log("setting up component version")
	cv := DefaultComponent.DeepCopy()
	cd := DefaultComponentDescriptor.DeepCopy()
	cv.Status.ComponentDescriptor = v1alpha1.Reference{
		Name:    obj.Spec.Resource.Name,
		Version: obj.Spec.Resource.Version,
		ComponentDescriptorRef: meta.NamespacedObjectReference{
			Name:      cd.Name,
			Namespace: cd.Namespace,
		},
	}

	client := env.FakeKubeClient(WithObjets(cv, obj, cd))
	t.Log("priming fake cache")
	cache := &cachefakes.FakeCache{}
	cache.PushDataReturns("digest", nil)

	t.Log("priming fake ocm client")
	ocmClient := &fakes.MockFetcher{}
	ocmClient.GetResourceReturns(io.NopCloser(bytes.NewBuffer([]byte("content"))), "digest", nil)
	recorder := record.NewFakeRecorder(32)

	rr := ResourceReconciler{
		Scheme:        env.scheme,
		Client:        client,
		OCMClient:     ocmClient,
		EventRecorder: recorder,
		Cache:         cache,
	}

	t.Log("calling reconcile on resource controller")
	_, err := rr.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: obj.Namespace,
			Name:      obj.Name,
		},
	})
	require.NoError(t, err)

	t.Log("verifying generated snapshot")
	snapshot := &v1alpha1.Snapshot{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      obj.Spec.SnapshotTemplate.Name,
		Namespace: obj.Namespace,
	}, snapshot)

	require.NoError(t, err)
	assert.Equal(t, "digest", snapshot.Spec.Digest)
	assert.Equal(t, "1.0.0", snapshot.Spec.Tag)

	t.Log("verifying updated resource object status")
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      obj.Name,
		Namespace: obj.Namespace,
	}, obj)

	require.NoError(t, err)
	assert.Equal(t, "1.0.0", obj.Status.LastAppliedResourceVersion)

	hash, err := snapshot.Spec.Identity.Hash()
	require.NoError(t, err)
	assert.Equal(t, "sha-18322151501422808564", hash)
	assert.True(t, conditions.IsTrue(obj, meta.ReadyCondition))

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

func TestResourceReconcilerFailed(t *testing.T) {
	t.Log("setting up resource object")
	obj := DefaultResource.DeepCopy()
	// Tests that the component descriptor exists for root items.
	obj.Spec.Resource.ReferencePath = nil

	t.Log("setting up component version")
	cv := DefaultComponent.DeepCopy()
	cd := DefaultComponentDescriptor.DeepCopy()
	cv.Status.ComponentDescriptor = v1alpha1.Reference{
		Name:    obj.Spec.Resource.Name,
		Version: obj.Spec.Resource.Version,
		ComponentDescriptorRef: meta.NamespacedObjectReference{
			Name:      cd.Name,
			Namespace: cd.Namespace,
		},
	}

	client := env.FakeKubeClient(WithObjets(cv, obj, cd))
	t.Log("priming fake cache")
	cache := &cachefakes.FakeCache{}
	cache.PushDataReturns("", errors.New("nope"))

	t.Log("priming fake ocm client")
	ocmClient := &fakes.MockFetcher{}
	ocmClient.GetResourceReturns(nil, "nil", errors.New("nope"))

	rr := ResourceReconciler{
		Scheme:        env.scheme,
		Client:        client,
		OCMClient:     ocmClient,
		EventRecorder: record.NewFakeRecorder(32),
		Cache:         cache,
	}

	t.Log("calling reconcile on resource controller")
	_, err := rr.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: obj.Namespace,
			Name:      obj.Name,
		},
	})
	assert.EqualError(t, err, "failed to get resource: nope")
	t.Log("verifying updated resource object status")
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      obj.Name,
		Namespace: obj.Namespace,
	}, obj)

	require.NoError(t, err)
	assert.True(t, conditions.IsFalse(obj, meta.ReadyCondition))
}

func TestResourceShouldReconcile(t *testing.T) {
	testcase := []struct {
		name             string
		errStr           string
		snapshot         func(resource v1alpha1.Resource) *v1alpha1.Snapshot
		componentVersion func() *v1alpha1.ComponentVersion
	}{
		{
			name: "should not reconcile in case of matching generation and existing snapshot with ready state",
			componentVersion: func() *v1alpha1.ComponentVersion {
				cv := DefaultComponent.DeepCopy()
				cv.Status.ReconciledVersion = "v0.0.1"
				return cv
			},
			snapshot: func(resource v1alpha1.Resource) *v1alpha1.Snapshot {
				snapshot := &v1alpha1.Snapshot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resource.Spec.SnapshotTemplate.Name,
						Namespace: resource.Namespace,
					},
					Spec:   v1alpha1.SnapshotSpec{},
					Status: v1alpha1.SnapshotStatus{},
				}
				conditions.MarkTrue(snapshot, meta.ReadyCondition, meta.SucceededReason, "Snapshot with name '%s' is ready", snapshot.Name)
				return snapshot
			},
		},
		{
			name:   "should reconcile if snapshot is not ready",
			errStr: "failed to get resource: unexpected number of calls; not enough return values have been configured; call count 0",
			componentVersion: func() *v1alpha1.ComponentVersion {
				cv := DefaultComponent.DeepCopy()
				cv.Status.ReconciledVersion = "v0.0.1"
				return cv
			},
			snapshot: func(resource v1alpha1.Resource) *v1alpha1.Snapshot {
				snapshot := &v1alpha1.Snapshot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resource.Spec.SnapshotTemplate.Name,
						Namespace: resource.Namespace,
					},
					Spec:   v1alpha1.SnapshotSpec{},
					Status: v1alpha1.SnapshotStatus{},
				}
				conditions.MarkFalse(snapshot, meta.ReadyCondition, meta.SucceededReason, "Snapshot with name '%s' is ready", snapshot.Name)
				return snapshot
			},
		},
		{
			name:   "should reconcile if component version doesn't match",
			errStr: "failed to get resource: unexpected number of calls; not enough return values have been configured; call count 0",
			componentVersion: func() *v1alpha1.ComponentVersion {
				cv := DefaultComponent.DeepCopy()
				cv.Status.ReconciledVersion = "v0.0.2"
				return cv
			},
			snapshot: func(resource v1alpha1.Resource) *v1alpha1.Snapshot {
				return nil
			},
		},
	}

	for i, tt := range testcase {
		t.Run(fmt.Sprintf("%d: %s", i, tt.name), func(t *testing.T) {
			// We don't set a source because it shouldn't get that far.
			obj := DefaultResource.DeepCopy()
			obj.Status.LastAppliedComponentVersion = "v0.0.1"
			snapshot := tt.snapshot(*obj)
			cv := tt.componentVersion()

			objs := []client.Object{cv, obj}
			if snapshot != nil {
				objs = append(objs, snapshot)
			}
			client := env.FakeKubeClient(WithObjets(objs...))
			cache := &cachefakes.FakeCache{}
			fakeOcm := &fakes.MockFetcher{}

			rr := ResourceReconciler{
				Client:        client,
				Scheme:        env.scheme,
				OCMClient:     fakeOcm,
				EventRecorder: record.NewFakeRecorder(32),
				Cache:         cache,
			}

			result, err := rr.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: obj.Namespace,
					Name:      obj.Name,
				},
			})

			if tt.errStr == "" {
				require.NoError(t, err)
				assert.Equal(t, ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, result)
				assert.True(t, cache.FetchDataByDigestWasNotCalled())
				assert.True(t, cache.PushDataWasNotCalled())
				assert.True(t, fakeOcm.GetResourceWasNotCalled())
			} else {
				assert.EqualError(t, err, tt.errStr)
			}
		})
	}
}

func TestResourceWithCreateFluxSource(t *testing.T) {
	testcase := []struct {
		name                string
		errStr              string
		componentDescriptor func() *v1alpha1.ComponentDescriptor
		componentVersion    func() *v1alpha1.ComponentVersion
		assert              func(t *testing.T, client client.Client, name string)
	}{
		{
			name: "should reconcile and create an OCIRepository with the given name in case of normal resource type",
			componentVersion: func() *v1alpha1.ComponentVersion {
				cv := DefaultComponent.DeepCopy()
				cv.Status.ReconciledVersion = "v0.0.1"
				return cv
			},
			componentDescriptor: func() *v1alpha1.ComponentDescriptor {
				return DefaultComponentDescriptor.DeepCopy()
			},
			assert: func(t *testing.T, client client.Client, name string) {
				t.Helper()

				snapshot := &v1alpha1.Snapshot{}
				assert.NoError(t, client.Get(context.Background(), types.NamespacedName{
					Namespace: "default",
					Name:      name,
				}, snapshot))

				assert.Equal(t, "ociImage", snapshot.GetContentType())
			},
		},
		{
			name: "should reconcile and create a HelmRepository with the given name in case of a helmChart",
			componentVersion: func() *v1alpha1.ComponentVersion {
				cv := DefaultComponent.DeepCopy()
				cv.Status.ReconciledVersion = "v0.0.1"
				return cv
			},
			componentDescriptor: func() *v1alpha1.ComponentDescriptor {
				cd := DefaultComponentDescriptor.DeepCopy()
				cd.Spec.Resources = []v3alpha1.Resource{
					{
						ElementMeta: v3alpha1.ElementMeta{
							Name:    "introspect-image",
							Version: "1.0.0",
						},
						Type:     "helmChart",
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
				}
				return cd
			},
			assert: func(t *testing.T, client client.Client, name string) {
				t.Helper()

				snapshot := &v1alpha1.Snapshot{}
				assert.NoError(t, client.Get(context.Background(), types.NamespacedName{
					Namespace: "default",
					Name:      name,
				}, snapshot))

				assert.Equal(t, "helmChart", snapshot.GetContentType())
			},
		},
	}

	for i, tt := range testcase {
		t.Run(fmt.Sprintf("%d: %s", i, tt.name), func(t *testing.T) {
			// We don't set a source because it shouldn't get that far.
			obj := DefaultResource.DeepCopy()
			obj.Spec.SnapshotTemplate.CreateFluxSource = true
			obj.Status.LastAppliedComponentVersion = "v0.0.1"
			cv := tt.componentVersion()
			cd := tt.componentDescriptor()
			cv.Status.ComponentDescriptor = v1alpha1.Reference{
				Name:    obj.Spec.Resource.Name,
				Version: obj.Spec.Resource.Version,
				ComponentDescriptorRef: meta.NamespacedObjectReference{
					Name:      cd.Name,
					Namespace: cd.Namespace,
				},
			}

			objs := []client.Object{cv, obj, cd}
			fakeKubeClient := env.FakeKubeClient(WithObjets(objs...))
			cache := &cachefakes.FakeCache{}
			fakeOcm := &fakes.MockFetcher{}
			fakeOcm.GetResourceReturns(io.NopCloser(bytes.NewBuffer([]byte("content"))), "digest", nil)

			rr := ResourceReconciler{
				Client:        fakeKubeClient,
				Scheme:        env.scheme,
				OCMClient:     fakeOcm,
				EventRecorder: record.NewFakeRecorder(32),
				Cache:         cache,
			}

			result, err := rr.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: obj.Namespace,
					Name:      obj.Name,
				},
			})

			require.NoError(t, err)
			assert.Equal(t, ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, result)
			assert.True(t, cache.FetchDataByDigestWasNotCalled())
			assert.True(t, cache.PushDataWasNotCalled())

			tt.assert(t, fakeKubeClient, obj.Spec.SnapshotTemplate.Name)
		})
	}
}
