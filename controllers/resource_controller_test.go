// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"

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
