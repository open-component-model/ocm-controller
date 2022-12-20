package controllers

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	cachefakes "github.com/open-component-model/ocm-controller/pkg/cache/fakes"
	"github.com/open-component-model/ocm-controller/pkg/ocm/fakes"
)

func TestResourceReconciler(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	assert.NoError(t, err)
	err = corev1.AddToScheme(scheme)
	assert.NoError(t, err)

	fakeClient := fake.NewClientBuilder()
	var (
		namespace = "default"
		component = "github.com/skarlso/test"
	)

	t.Log("setting up component version")
	cv := &v1alpha1.ComponentVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-component",
			Namespace: namespace,
		},
		Spec: v1alpha1.ComponentVersionSpec{
			Interval:  metav1.Duration{Duration: 10 * time.Minute},
			Component: component,
			Version: v1alpha1.Version{
				Semver: "v0.0.1",
			},
			Repository: v1alpha1.Repository{
				URL: "https://github.com/Skarlso/test",
			},
			Verify: []v1alpha1.Signature{},
			References: v1alpha1.ReferencesConfig{
				Expand: true,
			},
		},
		Status: v1alpha1.ComponentVersionStatus{},
	}

	t.Log("setting up resource object")
	obj := &v1alpha1.Resource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-resource",
			Namespace: "default",
		},
		Spec: v1alpha1.ResourceSpec{
			Interval: metav1.Duration{Duration: 10 * time.Minute},
			ComponentVersionRef: meta.NamespacedObjectReference{
				Name:      cv.Name,
				Namespace: cv.Namespace,
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
	client := fakeClient.WithObjects(cv, obj).WithScheme(scheme).Build()

	t.Log("priming fake cache")
	cache := &cachefakes.FakeCache{}
	cache.PushDataReturns("digest", nil)

	t.Log("priming fake ocm client")
	ocmClient := &fakes.MockFetcher{}
	ocmClient.GetResourceReturns(io.NopCloser(bytes.NewBuffer([]byte("content"))), nil)

	rr := ResourceReconciler{
		Scheme:    scheme,
		Client:    client,
		OCMClient: ocmClient,
		Cache:     cache,
	}

	t.Log("calling reconcile on resource controller")
	_, err = rr.reconcile(context.Background(), obj)
	require.NoError(t, err)

	t.Log("verifying generated snapshot")
	snapshot := &v1alpha1.Snapshot{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      obj.Spec.SnapshotTemplate.Name,
		Namespace: obj.Namespace,
	}, snapshot)

	require.NoError(t, err)
	assert.Equal(t, "digest", snapshot.Status.Digest)
	assert.Equal(t, "v0.0.1", snapshot.Status.Tag)

	t.Log("verifying updated resource object status")
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      obj.Name,
		Namespace: obj.Namespace,
	}, obj)

	require.NoError(t, err)
	assert.Equal(t, "v0.0.1", obj.Status.LastAppliedResourceVersion)

	t.Log("verifying calling parameters for cache")
	args := cache.PushDataCallingArgumentsOnCall(0)
	require.NotEmpty(t, args, "cache pushData should have been called once")
	assert.Equal(t, "content", args[0])

	hash, err := snapshot.Spec.Identity.Hash()
	require.NoError(t, err)
	assert.Equal(t, hash, args[1])
	assert.Equal(t, "v0.0.1", args[2])
}
