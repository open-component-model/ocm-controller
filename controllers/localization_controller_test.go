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

	ocmmetav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/ocm.software/v3alpha1"
	"github.com/open-component-model/ocm/pkg/runtime"

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
	cd := &v1alpha1.ComponentDescriptor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cv.Name + "-descriptor",
			Namespace: cv.Namespace,
		},
		Spec: v1alpha1.ComponentDescriptorSpec{
			ComponentVersionSpec: v3alpha1.ComponentVersionSpec{
				Resources: []v3alpha1.Resource{
					{
						ElementMeta: v3alpha1.ElementMeta{
							Name:    resource.Spec.Resource.Name,
							Version: resource.Spec.Resource.Version,
						},
						Type:     "ociImage",
						Relation: "local",
						Access: &runtime.UnstructuredTypedObject{
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
				},
			},
			Version: "v0.0.1",
		},
		Status: v1alpha1.ComponentDescriptorStatus{},
	}
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
	obj := &v1alpha1.Localization{
		TypeMeta: metav1.TypeMeta{
			Kind: "Localization",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-localization",
			Namespace: "default",
		},
		Spec: v1alpha1.LocalizationSpec{
			Interval: metav1.Duration{},
			Source: v1alpha1.Source{
				SourceRef: &meta.NamespacedObjectKindReference{
					APIVersion: v1alpha1.GroupVersion.String(),
					Kind:       "Snapshot",
					Name:       sourceSnapshot.Name,
					Namespace:  sourceSnapshot.Namespace,
				},
			},
			ConfigRef: v1alpha1.ConfigReference{
				ComponentVersionRef: meta.NamespacedObjectReference{
					Name:      cv.Name,
					Namespace: cv.Namespace,
				},
				Resource: v1alpha1.Source{
					ResourceRef: &v1alpha1.ResourceRef{
						Name: resource.Name,
					},
				},
			},
			SnapshotTemplate: v1alpha1.SnapshotTemplateSpec{
				Name: "test-localization-modified",
				Tag:  "v0.0.2",
			},
		},
	}
	client := env.FakeKubeClient(WithObjets(cv, resource, sourceSnapshot, cd, obj))
	t.Log("priming fake cache")
	cache := &cachefakes.FakeCache{}
	// set up the source snapshot bytes. this must be a TAR file.
	content, err := os.Open(filepath.Join("testdata", "localization-with-source.tar"))
	require.NoError(t, err)
	cache.FetchDataByDigestReturns(content, nil)

	t.Log("priming fake ocm")
	fakeOcm := &fakes.MockFetcher{}
	config := []byte(`kind: ConfigData
metadata:
  name: test-config-data
  namespace: default
localization:
- file: component-descriptor.yaml
  image: test-image-override
  repository: test-repo-override
  resource:
    name: test-resource
  tag: v0.0.8
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
	name, version := args[1], args[2]
	assert.Equal(t, "sha-6558931820223250200", name)
	assert.Equal(t, "999", version)
}
