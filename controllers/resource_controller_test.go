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
	ocmmetav1 "ocm.software/ocm/api/ocm/compdesc/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	cachefakes "github.com/open-component-model/ocm-controller/pkg/cache/fakes"
	"github.com/open-component-model/ocm-controller/pkg/ocm"
	"github.com/open-component-model/ocm-controller/pkg/ocm/fakes"
)

func TestResourceReconciler(t *testing.T) {
	t.Log("setting up resource object")
	resource := DefaultResource.DeepCopy()
	// Tests that the component descriptor exists for root items.
	resource.Spec.SourceRef.ResourceRef.ReferencePath = nil
	resource.Status.SnapshotName = "test-resource-lmt3orf"

	t.Log("setting up component version")
	cv := DefaultComponent.DeepCopy()
	cd := DefaultComponentDescriptor.DeepCopy()
	cv.Status.ComponentDescriptor = v1alpha1.Reference{
		Name:    resource.Spec.SourceRef.Name,
		Version: resource.Spec.SourceRef.GetVersion(),
		ComponentDescriptorRef: meta.NamespacedObjectReference{
			Name:      cd.Name,
			Namespace: cd.Namespace,
		},
	}
	conditions.MarkTrue(cv,
		meta.ReadyCondition,
		meta.SucceededReason,
		"Applied version: 1.0.0")

	client := env.FakeKubeClient(WithObjects(cv, resource, cd))
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
			Namespace: resource.Namespace,
			Name:      resource.Name,
		},
	})
	require.NoError(t, err)

	t.Log("verifying generated snapshot")
	snapshot := &v1alpha1.Snapshot{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      resource.Status.SnapshotName,
		Namespace: resource.Namespace,
	}, snapshot)

	require.NoError(t, err)
	assert.Equal(t, "digest", snapshot.Spec.Digest)
	assert.Equal(t, "1.0.0", snapshot.Spec.Tag)

	t.Log("verifying updated resource object status")
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      resource.Name,
		Namespace: resource.Namespace,
	}, resource)

	require.NoError(t, err)
	assert.Equal(t, "1.0.0", resource.Status.LastAppliedResourceVersion)

	hash, err := ocm.HashIdentity(snapshot.Spec.Identity)
	require.NoError(t, err)
	assert.Equal(t, "sha-18322151501422808564", hash)
	assert.True(t, conditions.IsTrue(resource, meta.ReadyCondition))

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

func TestResourceReconcilerWithReferencePath(t *testing.T) {
	t.Log("setting up resource object")
	resource := DefaultResource.DeepCopy()
	// Tests that the component descriptor exists for root items.
	resource.Spec.SourceRef.ResourceRef.ReferencePath = []ocmmetav1.Identity{
		{
			"name": resource.Spec.SourceRef.Name,
		},
	}
	resource.Status.SnapshotName = "test-resource-lmt3orf"

	t.Log("setting up component version")
	cv := DefaultComponent.DeepCopy()
	cd := DefaultComponentDescriptor.DeepCopy()
	cv.Status.ComponentDescriptor = v1alpha1.Reference{
		Name:    resource.Spec.SourceRef.Name,
		Version: resource.Spec.SourceRef.GetVersion(),
		ComponentDescriptorRef: meta.NamespacedObjectReference{
			Name:      cd.Name,
			Namespace: cd.Namespace,
		},
	}
	conditions.MarkTrue(cv,
		meta.ReadyCondition,
		meta.SucceededReason,
		"Applied version: 1.0.0")

	client := env.FakeKubeClient(WithObjects(cv, resource, cd))
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
			Namespace: resource.Namespace,
			Name:      resource.Name,
		},
	})
	require.NoError(t, err)

	t.Log("verifying generated snapshot")
	snapshot := &v1alpha1.Snapshot{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      resource.Status.SnapshotName,
		Namespace: resource.Namespace,
	}, snapshot)

	require.NoError(t, err)
	assert.Equal(t, "digest", snapshot.Spec.Digest)
	assert.Equal(t, "1.0.0", snapshot.Spec.Tag)

	t.Log("verifying updated resource object status")
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      resource.Name,
		Namespace: resource.Namespace,
	}, resource)

	require.NoError(t, err)
	assert.Equal(t, "1.0.0", resource.Status.LastAppliedResourceVersion)

	hash, err := ocm.HashIdentity(snapshot.Spec.Identity)
	require.NoError(t, err)
	assert.Equal(t, "sha-6345262111825580774", hash)
	assert.True(t, conditions.IsTrue(resource, meta.ReadyCondition))

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

func XTestResourceReconcilerFailed(t *testing.T) {
	t.Log("setting up resource object")
	resource := DefaultResource.DeepCopy()
	// Tests that the component descriptor exists for root items.
	resource.Spec.SourceRef.ResourceRef.ReferencePath = nil

	t.Log("setting up component version")
	cv := DefaultComponent.DeepCopy()
	cd := DefaultComponentDescriptor.DeepCopy()
	cv.Status.ComponentDescriptor = v1alpha1.Reference{
		Name:    resource.Spec.SourceRef.Name,
		Version: resource.Spec.SourceRef.GetVersion(),
		ComponentDescriptorRef: meta.NamespacedObjectReference{
			Name:      cd.Name,
			Namespace: cd.Namespace,
		},
	}
	conditions.MarkTrue(cv,
		meta.ReadyCondition,
		meta.SucceededReason,
		"Applied version: 1.0.0")

	client := env.FakeKubeClient(WithObjects(cv, resource, cd))
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
			Namespace: resource.Namespace,
			Name:      resource.Name,
		},
	})
	assert.EqualError(t, err, "failed to get resource: nope")
	t.Log("verifying updated resource object status")
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      resource.Name,
		Namespace: resource.Namespace,
	}, resource)

	require.NoError(t, err)
	assert.True(t, conditions.IsFalse(resource, meta.ReadyCondition))
}

func TestResourceReconcilerVersionDefaulting(t *testing.T) {
	t.Log("setting up resource object without explicit version")
	resource := DefaultResource.DeepCopy()
	resource.Spec.SourceRef.ResourceRef.ReferencePath = nil
	resource.Spec.SourceRef.ResourceRef.Version = ""
	resource.Status.SnapshotName = "test-resource-version-default"

	t.Log("setting up component version with different reconciled version")
	cv := DefaultComponent.DeepCopy()
	cv.Status.ReconciledVersion = "v0.9.9"

	t.Log("setting up component descriptor with different version")
	cd := DefaultComponentDescriptor.DeepCopy()
	cd.Spec.Version = "v1.2.3"

	cv.Status.ComponentDescriptor = v1alpha1.Reference{
		Name:    resource.Spec.SourceRef.Name,
		Version: resource.Spec.SourceRef.GetVersion(),
		ComponentDescriptorRef: meta.NamespacedObjectReference{
			Name:      cd.Name,
			Namespace: cd.Namespace,
		},
	}
	conditions.MarkTrue(cv,
		meta.ReadyCondition,
		meta.SucceededReason,
		"Applied version: v0.9.9")

	client := env.FakeKubeClient(WithObjects(cv, resource, cd))
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
			Namespace: resource.Namespace,
			Name:      resource.Name,
		},
	})
	require.NoError(t, err)

	t.Log("verifying generated snapshot uses component descriptor version")
	snapshot := &v1alpha1.Snapshot{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      resource.Status.SnapshotName,
		Namespace: resource.Namespace,
	}, snapshot)

	require.NoError(t, err)
	assert.Equal(t, "digest", snapshot.Spec.Digest)
	assert.Equal(t, "v1.2.3", snapshot.Spec.Tag)

	t.Log("verifying resource status uses component descriptor version")
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      resource.Name,
		Namespace: resource.Namespace,
	}, resource)

	require.NoError(t, err)
	assert.Equal(t, "v1.2.3", resource.Status.LastAppliedResourceVersion)
	assert.True(t, conditions.IsTrue(resource, meta.ReadyCondition))
}

// TODO: rewrite these so that they test the predicate functions.
func XTestResourceShouldReconcile(t *testing.T) {
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
						Name:      resource.Status.SnapshotName,
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
						Name:      resource.Status.SnapshotName,
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
			resource := DefaultResource.DeepCopy()
			resource.Status.LastAppliedComponentVersion = "v0.0.1"
			snapshot := tt.snapshot(*resource)
			cv := tt.componentVersion()

			objs := []client.Object{cv, resource}
			if snapshot != nil {
				objs = append(objs, snapshot)
			}
			client := env.FakeKubeClient(WithObjects(objs...))
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
					Namespace: resource.Namespace,
					Name:      resource.Name,
				},
			})

			if tt.errStr == "" {
				require.NoError(t, err)
				assert.Equal(t, ctrl.Result{RequeueAfter: resource.GetRequeueAfter()}, result)
				assert.True(t, cache.FetchDataByDigestWasNotCalled())
				assert.True(t, cache.PushDataWasNotCalled())
				assert.True(t, fakeOcm.GetResourceWasNotCalled())
			} else {
				assert.EqualError(t, err, tt.errStr)
			}
		})
	}
}
