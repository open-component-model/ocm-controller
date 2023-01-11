package controllers

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	cachefakes "github.com/open-component-model/ocm-controller/pkg/cache/fakes"
	"github.com/open-component-model/ocm-controller/pkg/ocm/fakes"
)

func TestResourceReconciler(t *testing.T) {
	t.Log("setting up component version")

	cv := DefaultComponent.DeepCopy()

	t.Log("setting up resource object")
	resource := DefaultResource.DeepCopy()

	client := env.FakeKubeClient(WithObjets(cv, resource))
	t.Log("priming fake cache")
	cache := &cachefakes.FakeCache{}
	cache.PushDataReturns("digest", nil)

	t.Log("priming fake ocm client")
	ocmClient := &fakes.MockFetcher{}
	ocmClient.GetResourceReturns(io.NopCloser(bytes.NewBuffer([]byte("content"))), nil)

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

	t.Log("verifying calling parameters for cache")
	args := cache.PushDataCallingArgumentsOnCall(0)
	require.NotEmpty(t, args, "cache pushData should have been called once")
	assert.Equal(t, "content", args[0])

	hash, err := snapshot.Spec.Identity.Hash()
	require.NoError(t, err)
	assert.Equal(t, hash, args[1])
	assert.Equal(t, "1.0.0", args[2])
}
