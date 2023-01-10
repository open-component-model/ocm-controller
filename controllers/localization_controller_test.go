package controllers

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	cachefakes "github.com/open-component-model/ocm-controller/pkg/cache/fakes"
	"github.com/open-component-model/ocm-controller/pkg/ocm/fakes"
)

func TestLocalizationReconcilerWithSourceRef(t *testing.T) {
	t.Log("setting up resource object")
	resource := DefaultResource.DeepCopy()

	t.Log("setting up component version")
	cv := DefaultComponent.DeepCopy()
	cv.Status.ComponentDescriptor = v1alpha1.Reference{
		Name:    "test-component",
		Version: "v0.0.1",
		ComponentDescriptorRef: meta.NamespacedObjectReference{
			Name:      cv.Name + "-descriptor",
			Namespace: cv.Namespace,
		},
	}
	cd := DefaultComponentDescriptor.DeepCopy()

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
	obj := DefaultLocalization.DeepCopy()
	obj.Spec.Source = v1alpha1.Source{
		SourceRef: &meta.NamespacedObjectKindReference{
			APIVersion: v1alpha1.GroupVersion.String(),
			Kind:       "Snapshot",
			Name:       sourceSnapshot.Name,
			Namespace:  sourceSnapshot.Namespace,
		},
	}

	client := env.FakeKubeClient(WithObjets(cv, resource, sourceSnapshot, cd, obj))
	t.Log("priming fake cache")
	cache := &cachefakes.FakeCache{}
	// set up the source snapshot bytes. this must be a TAR file.
	content, err := os.Open(filepath.Join("testdata", "localization-deploy.tar"))
	require.NoError(t, err)
	cache.FetchDataByDigestReturns(content, nil)

	t.Log("priming fake ocm")
	fakeOcm := &fakes.MockFetcher{}
	config := []byte(`kind: ConfigData
metadata:
  name: test-config-data
  namespace: default
localization:
- file: deploy.yaml
  image: spec.template.spec.containers[0].image
  resource:
    name: introspect-image
`)

	fakeOcm.GetResourceReturns(io.NopCloser(bytes.NewBuffer(config)), nil)

	lr := LocalizationReconciler{
		Client:    client,
		Scheme:    env.scheme,
		OCMClient: fakeOcm,
		Cache:     cache,
	}

	t.Log("start the reconcile loop")
	_, err = lr.reconcile(context.Background(), obj)
	require.NoError(t, err)

	t.Log("check if target snapshot has been created and cache was called")
	snapshotOutput := &v1alpha1.Snapshot{}
	err = client.Get(context.Background(), types.NamespacedName{
		Namespace: obj.Namespace,
		Name:      obj.Spec.SnapshotTemplate.Name,
	}, snapshotOutput)
	require.NoError(t, err)
	args := cache.PushDataCallingArgumentsOnCall(0)
	data, name, version := args[0], args[1], args[2]
	assert.Equal(t, "sha-6558931820223250200", name)
	assert.Equal(t, "999", version)

	t.Log("extracting the passed in data and checking if the localization worked")
	dataContent, err := Untar(io.NopCloser(bytes.NewBuffer([]byte(data.(string)))))
	require.NoError(t, err)
	assert.Contains(
		t,
		string(dataContent),
		"image: ghcr.io/mandelsoft/cnudie/component-descriptors/github.com/vasu1124/introspect@sha256:7f0168496f273c1e2095703a050128114d339c580b0906cd124a93b66ae471e2",
		"the image should have been altered during localization",
	)
}

func TestLocalizationReconcilerWithResourceRef(t *testing.T) {
	t.Log("setting up resource object")
	resource := DefaultResource.DeepCopy()

	t.Log("setting up component version")
	cv := DefaultComponent.DeepCopy()
	cv.Status.ComponentDescriptor = v1alpha1.Reference{
		Name:    "test-component",
		Version: "v0.0.1",
		ComponentDescriptorRef: meta.NamespacedObjectReference{
			Name:      cv.Name + "-descriptor",
			Namespace: cv.Namespace,
		},
	}
	cd := DefaultComponentDescriptor.DeepCopy()

	obj := DefaultLocalization.DeepCopy()
	obj.Spec.Source = v1alpha1.Source{
		ResourceRef: &v1alpha1.ResourceRef{
			Name:    "some-resource",
			Version: "1.0.0",
		},
	}

	client := env.FakeKubeClient(WithObjets(cv, resource, cd, obj))
	t.Log("priming fake cache")
	cache := &cachefakes.FakeCache{}
	// set up the source snapshot bytes. this must be a TAR file.
	content, err := os.Open(filepath.Join("testdata", "localization-deploy.tar"))
	require.NoError(t, err)
	//cache.FetchDataByDigestReturns(content, nil)

	t.Log("priming fake ocm")
	fakeOcm := &fakes.MockFetcher{}

	fakeOcm.GetResourceReturnsOnCall(0, content, nil)
	config := []byte(`kind: ConfigData
metadata:
  name: test-config-data
  namespace: default
localization:
- file: deploy.yaml
  image: spec.template.spec.containers[0].image
  resource:
    name: introspect-image
`)

	fakeOcm.GetResourceReturnsOnCall(1, io.NopCloser(bytes.NewBuffer(config)), nil)

	lr := LocalizationReconciler{
		Client:    client,
		Scheme:    env.scheme,
		OCMClient: fakeOcm,
		Cache:     cache,
	}

	t.Log("start the reconcile loop")
	_, err = lr.reconcile(context.Background(), obj)
	require.NoError(t, err)

	t.Log("check if target snapshot has been created and cache was called")
	snapshotOutput := &v1alpha1.Snapshot{}
	err = client.Get(context.Background(), types.NamespacedName{
		Namespace: obj.Namespace,
		Name:      obj.Spec.SnapshotTemplate.Name,
	}, snapshotOutput)
	require.NoError(t, err)
	args := cache.PushDataCallingArgumentsOnCall(0)
	data, name, version := args[0], args[1], args[2]
	assert.Equal(t, "sha-6558931820223250200", name)
	assert.Equal(t, "999", version)

	t.Log("extracting the passed in data and checking if the localization worked")
	dataContent, err := Untar(io.NopCloser(bytes.NewBuffer([]byte(data.(string)))))
	require.NoError(t, err)
	assert.Contains(
		t,
		string(dataContent),
		"image: ghcr.io/mandelsoft/cnudie/component-descriptors/github.com/vasu1124/introspect@sha256:7f0168496f273c1e2095703a050128114d339c580b0906cd124a93b66ae471e2",
		"the image should have been altered during localization",
	)
}
