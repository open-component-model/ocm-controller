package controllers

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	cachefakes "github.com/open-component-model/ocm-controller/pkg/cache/fakes"
	"github.com/open-component-model/ocm-controller/pkg/ocm/fakes"
)

func TestResourceReconciler(t *testing.T) {
	t.Log("setting up resource object")
	resource := DefaultResource.DeepCopy()
	// Tests that the component descriptor exists for root items.
	resource.Spec.Resource.ReferencePath = nil

	t.Log("setting up component version")
	cv := DefaultComponent.DeepCopy()
	cd := DefaultComponentDescriptor.DeepCopy()
	cv.Status.ComponentDescriptor = v1alpha1.Reference{
		Name:    resource.Spec.Resource.Name,
		Version: resource.Spec.Resource.Version,
		ComponentDescriptorRef: meta.NamespacedObjectReference{
			Name:      cd.Name,
			Namespace: cd.Namespace,
		},
	}

	client := env.FakeKubeClient(WithObjets(cv, resource, cd))
	t.Log("priming fake cache")
	cache := &cachefakes.FakeCache{}
	cache.PushDataReturns("digest", nil)

	t.Log("priming fake ocm client")
	ocmClient := &fakes.MockFetcher{}
	ocmClient.GetResourceReturns(io.NopCloser(bytes.NewBuffer([]byte("content"))), "digest", nil)

	rr := ResourceReconciler{
		Scheme:    env.scheme,
		Client:    client,
		OCMClient: ocmClient,
		Cache:     cache,
	}

	t.Log("calling reconcile on resource controller")
	_, err := rr.reconcile(context.Background(), resource)
	require.NoError(t, err)

	t.Log("verifying generated snapshot")
	snapshot := &v1alpha1.Snapshot{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      resource.Spec.SnapshotTemplate.Name,
		Namespace: resource.Namespace,
	}, snapshot)

	require.NoError(t, err)
	assert.Equal(t, "digest", snapshot.Status.Digest)
	assert.Equal(t, "1.0.0", snapshot.Status.Tag)

	t.Log("verifying updated resource object status")
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      resource.Name,
		Namespace: resource.Namespace,
	}, resource)

	require.NoError(t, err)
	assert.Equal(t, "1.0.0", resource.Status.LastAppliedResourceVersion)

	hash, err := snapshot.Spec.Identity.Hash()
	require.NoError(t, err)
	assert.Equal(t, "sha-18322151501422808564", hash)
}
