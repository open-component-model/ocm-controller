package ocm

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/distribution/distribution/v3/registry/storage/driver/inmemory"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.podman.io/image/v5/pkg/compression"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"ocm.software/ocm/api/ocm"
	"ocm.software/ocm/api/ocm/compdesc"
	ocmmetav1 "ocm.software/ocm/api/ocm/compdesc/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"ocm.software/ocm/api/credentials"
	"ocm.software/ocm/api/credentials/cpi"
	"ocm.software/ocm/api/ocm/resolvers"
	"ocm.software/ocm/api/ocm/tools/signing"
	"ocm.software/ocm/api/tech/oci/identity"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/pkg/cache/fakes"
	fakeocm "github.com/open-component-model/ocm-controller/pkg/fakes"
)

func TestClient_GetResource(t *testing.T) {
	component := "ocm.software/ocm-demo-index"
	resource := "remote-controller-demo"
	resourceVersion := "v0.0.1"
	data := "testdata"

	octx := fakeocm.NewFakeOCMContext()

	comp := &fakeocm.Component{
		Name:    component,
		Version: "v0.0.1",
	}
	res := &fakeocm.Resource[*ocm.ResourceMeta]{
		Name:      resource,
		Version:   resourceVersion,
		Data:      []byte(data),
		Component: comp,
		Kind:      "localBlob",
		Type:      "ociBlob",
	}
	comp.Resources = append(comp.Resources, res)

	_ = octx.AddComponent(comp)

	cd := &v1alpha1.ComponentDescriptor{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "github.com-open-component-model-ocm-demo-index-v0.0.1-12345",
		},
		Spec: v1alpha1.ComponentDescriptorSpec{
			Version: "v0.0.1",
		},
	}

	fakeKubeClient := env.FakeKubeClient(WithObjects(cd))
	cache := &fakes.FakeCache{}
	cache.IsCachedReturns(false, nil)
	cache.FetchDataByDigestReturns(io.NopCloser(strings.NewReader("mockdata")), nil)
	cache.PushDataReturns("sha256:8fa155245ea8d3f2ea3add7d090d42dfb0e22799018fded6aae24f0c1a1c3f38", nil)

	ocmClient := NewClient(fakeKubeClient, cache)

	cv := &v1alpha1.ComponentVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-name",
			Namespace: "default",
		},
		Spec: v1alpha1.ComponentVersionSpec{
			Component: component,
			Version: v1alpha1.Version{
				Semver: "v0.0.1",
			},
			Repository: v1alpha1.Repository{
				URL: "localhost",
			},
		},
		Status: v1alpha1.ComponentVersionStatus{
			ReconciledVersion: "v0.0.1",
			ComponentDescriptor: v1alpha1.Reference{
				Name:    component,
				Version: "v0.0.1",
				ComponentDescriptorRef: meta.NamespacedObjectReference{
					Name:      "github.com-open-component-model-ocm-demo-index-v0.0.1-12345",
					Namespace: "default",
				},
			},
		},
	}

	resourceRef := &v1alpha1.ResourceReference{
		ElementMeta: v1alpha1.ElementMeta{
			Name:    "remote-controller-demo",
			Version: "v0.0.1",
		},
	}

	reader, digest, _, err := ocmClient.GetResource(context.Background(), octx, cv, resourceRef)
	assert.NoError(t, err)
	content, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, "mockdata", string(content))
	assert.Equal(t, "sha256:8fa155245ea8d3f2ea3add7d090d42dfb0e22799018fded6aae24f0c1a1c3f38", digest)

	// verify that the cache has been called with the right resource data to cache.
	args := cache.PushDataCallingArgumentsOnCall(0)

	assert.Equal(t, data, args.Content)

	assert.Equal(t, "sha-14469167939644886767", args.Name, "pushed name did not match constructed name from identity of the resource")
	assert.Equal(t, resourceRef.Version, args.Version)
}

func TestClient_GetResourceFromNestedComponent(t *testing.T) {
	component := "ocm.software/ocm-demo-index"
	component2 := "ocm.software/ocm-demo-index-2"
	component3 := "ocm.software/ocm-demo-index-3"
	resource := "remote-controller-demo"
	resourceVersion := "v0.0.1"
	data := "testdata"

	octx := fakeocm.NewFakeOCMContext()

	comp := &fakeocm.Component{
		Name:    component,
		Version: "v0.0.1",
		References: map[string]ocm.ComponentReference{
			string(ocmmetav1.Identity{"name": "nested-1"}.Digest()): {
				ElementMeta: compdesc.ElementMeta{
					Version: "v0.0.1",
					Name:    component,
				},
				ComponentName: component2,
				Digest:        nil,
			},
		},
	}
	comp2 := &fakeocm.Component{
		Name:    component2,
		Version: "v0.0.1",
		References: map[string]ocm.ComponentReference{
			string(ocmmetav1.Identity{"name": "nested-2"}.Digest()): {
				ElementMeta: compdesc.ElementMeta{
					Version: "v0.0.1",
					Name:    component2,
				},
				ComponentName: component3,
				Digest:        nil,
			},
		},
	}

	comp3 := &fakeocm.Component{
		Name:    component3,
		Version: "v0.0.1",
	}

	res := &fakeocm.Resource[*ocm.ResourceMeta]{
		Name:      resource,
		Version:   resourceVersion,
		Data:      []byte(data),
		Component: comp3,
		Kind:      "localBlob",
		Type:      "ociBlob",
	}
	comp3.Resources = append(comp3.Resources, res)

	_ = octx.AddComponent(comp)
	_ = octx.AddComponent(comp2)
	_ = octx.AddComponent(comp3)

	cd := &v1alpha1.ComponentDescriptor{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "github.com-open-component-model-ocm-demo-index-v0.0.1-12345",
		},
		Spec: v1alpha1.ComponentDescriptorSpec{
			Version: "v0.0.1",
		},
	}

	cd2 := &v1alpha1.ComponentDescriptor{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "github.com-open-component-model-ocm-demo-index-2-v0.0.1-12345",
		},
		Spec: v1alpha1.ComponentDescriptorSpec{
			Version: "v0.0.1",
		},
	}

	cd3 := &v1alpha1.ComponentDescriptor{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "github.com-open-component-model-ocm-demo-index-3-v0.0.1-12345",
		},
		Spec: v1alpha1.ComponentDescriptorSpec{
			Version: "v0.0.1",
		},
	}

	fakeKubeClient := env.FakeKubeClient(WithObjects(cd, cd2, cd3))
	cache := &fakes.FakeCache{}
	cache.IsCachedReturns(false, nil)
	cache.FetchDataByDigestReturns(io.NopCloser(strings.NewReader("mockdata")), nil)
	cache.PushDataReturns("sha256:8fa155245ea8d3f2ea3add7d090d42dfb0e22799018fded6aae24f0c1a1c3f38", nil)

	ocmClient := NewClient(fakeKubeClient, cache)

	cv := &v1alpha1.ComponentVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-name",
			Namespace: "default",
		},
		Spec: v1alpha1.ComponentVersionSpec{
			Component: component,
			Version: v1alpha1.Version{
				Semver: "v0.0.1",
			},
			Repository: v1alpha1.Repository{
				URL: "localhost",
			},
		},
		Status: v1alpha1.ComponentVersionStatus{
			ReconciledVersion: "v0.0.1",
			ComponentDescriptor: v1alpha1.Reference{
				Name:    component,
				Version: "v0.0.1",
				References: []v1alpha1.Reference{
					{
						Name:    "nested-1",
						Version: "v0.0.1",
						References: []v1alpha1.Reference{
							{
								Name:    "nested-2",
								Version: "v0.0.1",
								ComponentDescriptorRef: meta.NamespacedObjectReference{
									Name:      "github.com-open-component-model-ocm-demo-index-3-v0.0.1-12345",
									Namespace: "default",
								},
							},
						},
						ComponentDescriptorRef: meta.NamespacedObjectReference{
							Name:      "github.com-open-component-model-ocm-demo-index-2-v0.0.1-12345",
							Namespace: "default",
						},
					},
				},
				ComponentDescriptorRef: meta.NamespacedObjectReference{
					Name:      "github.com-open-component-model-ocm-demo-index-v0.0.1-12345",
					Namespace: "default",
				},
			},
		},
	}

	resourceRef := &v1alpha1.ResourceReference{
		ElementMeta: v1alpha1.ElementMeta{
			Name:    "remote-controller-demo",
			Version: "v0.0.1",
		},
		ReferencePath: []ocmmetav1.Identity{
			{
				"name": "nested-1",
			},
			{
				"name": "nested-2",
			},
		},
	}

	reader, digest, _, err := ocmClient.GetResource(context.Background(), octx, cv, resourceRef)
	require.NoError(t, err)
	content, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, "mockdata", string(content))
	assert.Equal(t, "sha256:8fa155245ea8d3f2ea3add7d090d42dfb0e22799018fded6aae24f0c1a1c3f38", digest)

	// verify that the cache has been called with the right resource data to cache.
	args := cache.PushDataCallingArgumentsOnCall(0)

	assert.Equal(t, data, args.Content)

	assert.Equal(t, "sha-8351589537464398024", args.Name, "pushed name did not match constructed name from identity of the resource")
	assert.Equal(t, resourceRef.Version, args.Version)
}

func TestClient_GetHelmResource(t *testing.T) {
	component := "ocm.software/ocm-demo-index"
	resource := "remote-controller-demo"
	resourceVersion := "v0.0.1"
	data, err := os.ReadFile(filepath.Join("testdata", "podinfo-6.3.5.tgz"))
	require.NoError(t, err)

	octx := fakeocm.NewFakeOCMContext()

	comp := &fakeocm.Component{
		Name:    component,
		Version: "v0.0.1",
	}
	res := &fakeocm.Resource[*ocm.ResourceMeta]{
		Name:      resource,
		Version:   resourceVersion,
		Data:      data,
		Component: comp,
		Kind:      "helmChart",
		Type:      "helm",
	}
	comp.Resources = append(comp.Resources, res)

	_ = octx.AddComponent(comp)

	cd := &v1alpha1.ComponentDescriptor{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "github.com-open-component-model-ocm-demo-index-v0.0.1-12345",
		},
		Spec: v1alpha1.ComponentDescriptorSpec{
			Version: "v0.0.1",
		},
	}

	fakeKubeClient := env.FakeKubeClient(WithObjects(cd))
	cache := &fakes.FakeCache{}
	cache.IsCachedReturns(false, nil)
	cache.FetchDataByDigestReturns(io.NopCloser(strings.NewReader("mockdata")), nil)
	cache.PushDataReturns("sha256:8fa155245ea8d3f2ea3add7d090d42dfb0e22799018fded6aae24f0c1a1c3f38", nil)

	ocmClient := NewClient(fakeKubeClient, cache)

	cv := &v1alpha1.ComponentVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-name",
			Namespace: "default",
		},
		Spec: v1alpha1.ComponentVersionSpec{
			Component: component,
			Version: v1alpha1.Version{
				Semver: "v0.0.1",
			},
			Repository: v1alpha1.Repository{
				URL: "localhost",
			},
		},
		Status: v1alpha1.ComponentVersionStatus{
			ReconciledVersion: "v0.0.1",
			ComponentDescriptor: v1alpha1.Reference{
				Name:    component,
				Version: "v0.0.1",
				ComponentDescriptorRef: meta.NamespacedObjectReference{
					Name:      "github.com-open-component-model-ocm-demo-index-v0.0.1-12345",
					Namespace: "default",
				},
			},
		},
	}

	resourceRef := &v1alpha1.ResourceReference{
		ElementMeta: v1alpha1.ElementMeta{
			Name:    "remote-controller-demo",
			Version: "v0.0.1",
		},
	}

	reader, digest, _, err := ocmClient.GetResource(context.Background(), octx, cv, resourceRef)
	assert.NoError(t, err)
	content, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, "mockdata", string(content))
	assert.Equal(t, "sha256:8fa155245ea8d3f2ea3add7d090d42dfb0e22799018fded6aae24f0c1a1c3f38", digest)

	// verify that the cache has been called with the right resource data to cache.
	args := cache.PushDataCallingArgumentsOnCall(0)

	decompressedDataReader, _, err := compression.AutoDecompress(bytes.NewBuffer(data))
	require.NoError(t, err)
	decompressedData, err := io.ReadAll(decompressedDataReader)
	require.NoError(t, err)
	assert.Equal(t, string(decompressedData), args.Content)

	assert.Equal(t, "sha-14469167939644886767", args.Name, "pushed name did not match constructed name from identity of the resource")
	assert.Equal(t, resourceRef.Version, args.Version)
}

func TestClient_GetComponentVersion(t *testing.T) {
	component := "ocm.software/ocm-demo-index"
	octx := fakeocm.NewFakeOCMContext()
	comp := &fakeocm.Component{
		Name:    component,
		Version: "v0.0.1",
	}

	require.NoError(t, octx.AddComponent(comp))

	fakeKubeClient := env.FakeKubeClient()
	cache := &fakes.FakeCache{}
	ocmClient := NewClient(fakeKubeClient, cache)

	cv := &v1alpha1.ComponentVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-name",
			Namespace: "default",
		},
		Spec: v1alpha1.ComponentVersionSpec{
			Component: component,
			Version: v1alpha1.Version{
				Semver: "v0.0.1",
			},
			Repository: v1alpha1.Repository{
				URL: "localhost",
			},
		},
		Status: v1alpha1.ComponentVersionStatus{
			ReconciledVersion:       "v0.0.1",
			ReplicatedRepositoryURL: "localhost",
		},
	}

	cva, err := ocmClient.GetComponentVersion(context.Background(), octx, cv.GetRepositoryURL(), component, "v0.0.1")
	assert.NoError(t, err)
	assert.Equal(t, cv.Spec.Component, cva.GetName())
}

func TestClient_CreateAuthenticatedOCMContextWithSecret(t *testing.T) {
	component := "ocm.software/ocm-demo-index"
	cs := &v1alpha1.ComponentVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-name",
			Namespace: "default",
		},
		Spec: v1alpha1.ComponentVersionSpec{
			Component: component,
			Repository: v1alpha1.Repository{
				URL: "localhost",
				SecretRef: &corev1.LocalObjectReference{
					Name: "test-secret",
				},
			},
		},
	}

	testSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"token":    []byte("token"),
			"username": []byte("username"),
			"password": []byte("password"),
		},
		Type: corev1.SecretTypeDockerConfigJson,
	}

	fakeKubeClient := env.FakeKubeClient(WithObjects(cs, testSecret))
	cache := &fakes.FakeCache{}
	ocmClient := NewClient(fakeKubeClient, cache)

	octx, err := ocmClient.CreateAuthenticatedOCMContext(context.Background(), cs)
	require.NoError(t, err)

	id := cpi.ConsumerIdentity{
		cpi.ID_TYPE:            identity.CONSUMER_TYPE,
		identity.ID_HOSTNAME:   "localhost",
		identity.ID_PATHPREFIX: "open-component-model",
	}

	creds, err := octx.CredentialsContext().GetCredentialsForConsumer(id)
	require.NoError(t, err)
	consumer, err := creds.Credentials(nil)
	require.NoError(t, err)

	assert.Equal(t, "password", consumer.Properties()["password"])
	assert.Equal(t, "token", consumer.Properties()["token"])
	assert.Equal(t, "username", consumer.Properties()["username"])
}

func TestClient_CreateAuthenticatedOCMContextWithServiceAccount(t *testing.T) {
	cs := &v1alpha1.ComponentVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-name",
			Namespace: "default",
		},
		Spec: v1alpha1.ComponentVersionSpec{
			Component: "ocm.software/ocm-demo-index",
			Repository: v1alpha1.Repository{
				URL: "localhost",
			},
			ServiceAccountName: "test-service-account",
		},
	}
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service-account",
			Namespace: "default",
		},
		ImagePullSecrets: []corev1.LocalObjectReference{
			{
				Name: "test-name-secret",
			},
		},
	}
	testSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-name-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			".dockerconfigjson": []byte(`{
  "auths": {
    "localhost": {
      "username": "open-component-model",
      "password": "password",
      "auth": "b3Blbi1jb21wb25lbnQtbW9kZWw6cGFzc3dvcmQ="
    }
  }
}`),
		},
		Type: corev1.SecretTypeDockerConfigJson,
	}

	fakeKubeClient := env.FakeKubeClient(WithObjects(cs, serviceAccount, testSecret))
	cache := &fakes.FakeCache{}
	ocmClient := NewClient(fakeKubeClient, cache)
	octx, err := ocmClient.CreateAuthenticatedOCMContext(context.Background(), cs)
	require.NoError(t, err)

	id := cpi.ConsumerIdentity{
		cpi.ID_TYPE:            identity.CONSUMER_TYPE,
		identity.ID_HOSTNAME:   "localhost",
		identity.ID_PATHPREFIX: "open-component-model",
	}
	creds, err := octx.CredentialsContext().GetCredentialsForConsumer(id)
	require.NoError(t, err)
	consumer, err := creds.Credentials(nil)
	require.NoError(t, err)

	assert.Equal(t, "password", consumer.Properties()["password"])
	assert.Equal(t, "open-component-model", consumer.Properties()["username"])
	assert.Equal(t, "localhost", consumer.Properties()["serverAddress"])
}

func TestClient_GetLatestValidComponentVersion(t *testing.T) {
	publicKey1, err := os.ReadFile(filepath.Join("testdata", "public1_key.pem"))
	require.NoError(t, err)
	privateKey, err := os.ReadFile(filepath.Join("testdata", "private_key.pem"))
	require.NoError(t, err)

	testCases := []struct {
		name             string
		componentVersion func(name string) *v1alpha1.ComponentVersion
		setupComponents  func(name string, context *fakeocm.Context)
		expectedVersion  string
	}{
		{
			name: "semver constraint works for greater versions",
			componentVersion: func(name string) *v1alpha1.ComponentVersion {
				return &v1alpha1.ComponentVersion{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-name",
						Namespace: "default",
					},
					Spec: v1alpha1.ComponentVersionSpec{
						Component: name,
						Version: v1alpha1.Version{
							Semver: ">v0.0.1",
						},
						Repository: v1alpha1.Repository{
							URL: "localhost",
						},
					},
				}
			},
			setupComponents: func(name string, context *fakeocm.Context) {
				// v0.0.1 should not be chosen.
				for _, v := range []string{"v0.0.1", "v0.0.5"} {
					_ = context.AddComponent(&fakeocm.Component{
						Name:    name,
						Version: v,
					})
				}
			},
			expectedVersion: "v0.0.5",
		},
		{
			name: "semver is a concrete match with multiple versions",
			componentVersion: func(name string) *v1alpha1.ComponentVersion {
				return &v1alpha1.ComponentVersion{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-name",
						Namespace: "default",
					},
					Spec: v1alpha1.ComponentVersionSpec{
						Component: name,
						Version: v1alpha1.Version{
							Semver: "v0.0.1",
						},
						Repository: v1alpha1.Repository{
							URL: "localhost",
						},
					},
				}
			},
			setupComponents: func(name string, context *fakeocm.Context) {
				for _, v := range []string{"v0.0.1", "v0.0.2", "v0.0.3"} {
					_ = context.AddComponent(&fakeocm.Component{
						Name:    name,
						Version: v,
					})
				}
			},
			expectedVersion: "v0.0.1",
		},
		{
			name: "semver is in between available versions should return the one that's still matching instead of the latest available",
			componentVersion: func(name string) *v1alpha1.ComponentVersion {
				return &v1alpha1.ComponentVersion{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-name",
						Namespace: "default",
					},
					Spec: v1alpha1.ComponentVersionSpec{
						Component: name,
						Version: v1alpha1.Version{
							Semver: "<=v0.0.2",
						},
						Repository: v1alpha1.Repository{
							URL: "localhost",
						},
					},
				}
			},
			setupComponents: func(name string, context *fakeocm.Context) {
				for _, v := range []string{"v0.0.1", "v0.0.2", "v0.0.3"} {
					_ = context.AddComponent(&fakeocm.Component{
						Name:    name,
						Version: v,
					})
				}
			},
			expectedVersion: "v0.0.2",
		},
		{
			name: "using = should still work as expected",
			componentVersion: func(name string) *v1alpha1.ComponentVersion {
				return &v1alpha1.ComponentVersion{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-name",
						Namespace: "default",
					},
					Spec: v1alpha1.ComponentVersionSpec{
						Component: name,
						Version: v1alpha1.Version{
							Semver: "=v0.0.1",
						},
						Repository: v1alpha1.Repository{
							URL: "localhost",
						},
					},
				}
			},
			setupComponents: func(name string, context *fakeocm.Context) {
				for _, v := range []string{"v0.0.1", "v0.0.2"} {
					_ = context.AddComponent(&fakeocm.Component{
						Name:    name,
						Version: v,
					})
				}
			},

			expectedVersion: "v0.0.1",
		},
		{
			name: "we are able to skip a version",
			componentVersion: func(name string) *v1alpha1.ComponentVersion {
				return &v1alpha1.ComponentVersion{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-name",
						Namespace: "default",
					},
					Spec: v1alpha1.ComponentVersionSpec{
						Component: name,
						Version: v1alpha1.Version{
							Semver: "!=v0.0.4",
						},
						Repository: v1alpha1.Repository{
							URL: "localhost",
						},
					},
				}
			},
			setupComponents: func(name string, context *fakeocm.Context) {
				for _, v := range []string{"v0.0.1", "v0.0.2", "v0.0.4", "v0.0.5"} {
					_ = context.AddComponent(&fakeocm.Component{
						Name:    name,
						Version: v,
					})
				}
			},

			expectedVersion: "v0.0.5",
		},
		{
			name: "latest _verified_ version is returned",
			componentVersion: func(name string) *v1alpha1.ComponentVersion {
				return &v1alpha1.ComponentVersion{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-name",
						Namespace: "default",
					},
					Spec: v1alpha1.ComponentVersionSpec{
						Component: name,
						Version: v1alpha1.Version{
							Semver: ">=v0.0.1",
						},
						Repository: v1alpha1.Repository{
							URL: "localhost",
						},
						Verify: []v1alpha1.Signature{
							{
								Name: Signature,
								PublicKey: v1alpha1.PublicKey{
									SecretRef: &corev1.LocalObjectReference{
										Name: "sign-secret",
									},
								},
							},
						},
					},
				}
			},
			setupComponents: func(name string, context *fakeocm.Context) {
				for _, v := range []string{"v0.0.1", "v0.0.2", "v0.0.4", "v0.0.5"} {
					if v == "v0.0.4" {
						// sign it
						_ = context.AddComponent(&fakeocm.Component{
							Name:    name,
							Version: v,
							Sign: &fakeocm.Sign{
								Name:    Signature,
								PrivKey: privateKey,
								PubKey:  publicKey1,
								Digest:  "3d879ecdea45acb7f8d85b89fd653288d84af4476eac4141822142ec59c13745",
							},
						})

						continue
					}

					_ = context.AddComponent(&fakeocm.Component{
						Name:    name,
						Version: v,
					})
				}
			},

			expectedVersion: "v0.0.4", // v0.0.4 is the only signed version and should be returned.
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()

			secretName := "sign-secret"
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: "default",
				},
				Data: map[string][]byte{
					Signature: publicKey1,
				},
			}

			fakeKubeClient := env.FakeKubeClient(WithObjects(secret))
			cache := &fakes.FakeCache{}
			ocmClient := NewClient(fakeKubeClient, cache)
			octx := fakeocm.NewFakeOCMContext()
			component := "ocm.software/ocm-demo-index"

			tt.setupComponents(component, octx)
			cv := tt.componentVersion(component)

			latest, err := ocmClient.GetLatestValidComponentVersion(context.Background(), octx, cv)
			require.NoError(t, err)
			require.NotNil(t, latest)
			defer latest.Close()
			assert.Equal(t, tt.expectedVersion, latest.GetVersion())
		})
	}
}

type signatureKeyLookupClient struct {
	client.Client
	gets int
}

func (c *signatureKeyLookupClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	c.gets++
	if c.Client == nil {
		return errors.New("unexpected public key lookup")
	}
	return c.Client.Get(ctx, key, obj, opts...)
}

func TestClient_GetLatestValidComponentVersionCachesKeysPerSelection(t *testing.T) {
	publicKey, err := os.ReadFile(filepath.Join("testdata", "public1_key.pem"))
	require.NoError(t, err)
	otherPublicKey, err := os.ReadFile(filepath.Join("testdata", "public2_key.pem"))
	require.NoError(t, err)
	privateKey, err := os.ReadFile(filepath.Join("testdata", "private_key.pem"))
	require.NoError(t, err)

	for _, tt := range []struct {
		name          string
		separateKeys  bool
		missingSecret bool
		gets          int
	}{
		{name: "shared Secret", gets: 1},
		{name: "separate Secrets", separateKeys: true, gets: 2},
		{name: "missing Secret is retried on the next selection", missingSecret: true, gets: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			octx := fakeocm.NewFakeOCMContext()
			for _, version := range []string{"1.0.0", "2.0.0"} {
				component := &fakeocm.Component{
					Name: "example.com/component", Version: version,
					Sign: &fakeocm.Sign{Name: "first", PrivKey: privateKey, PubKey: publicKey},
				}
				require.NoError(t, octx.AddComponent(component))
				second := component.ComponentDescriptor.Signatures[0]
				second.Name = "second"
				component.ComponentDescriptor.Signatures = append(component.ComponentDescriptor.Signatures, second)
			}
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "signing-keys", Namespace: "default"},
				Data:       map[string][]byte{"first": publicKey, "second": publicKey},
			}
			kubeClient := &signatureKeyLookupClient{Client: env.FakeKubeClient()}
			if !tt.missingSecret {
				require.NoError(t, kubeClient.Create(t.Context(), secret))
			}
			obj := &v1alpha1.ComponentVersion{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Spec: v1alpha1.ComponentVersionSpec{
					Component: "example.com/component", Repository: v1alpha1.Repository{URL: "localhost"},
					Version: v1alpha1.Version{Semver: ">=1.0.0"},
				},
			}
			for _, name := range []string{"first", "second"} {
				obj.Spec.Verify = append(obj.Spec.Verify, v1alpha1.Signature{
					Name: name, PublicKey: v1alpha1.PublicKey{SecretRef: &corev1.LocalObjectReference{Name: secret.Name}},
				})
			}
			if tt.separateKeys {
				secondSecret := secret.DeepCopy()
				secondSecret.Name = "other-signing-keys"
				secondSecret.ResourceVersion = ""
				require.NoError(t, kubeClient.Create(t.Context(), secondSecret))
				obj.Spec.Verify[1].PublicKey.SecretRef.Name = secondSecret.Name
			}
			ocmClient := NewClient(kubeClient, nil)
			selected, err := ocmClient.GetLatestValidComponentVersion(t.Context(), octx, obj)
			if tt.missingSecret {
				// a key that cannot be loaded is a hard failure, not a silent skip
				require.ErrorIs(t, err, ErrVerificationFailed)
				require.Nil(t, selected)
				require.NoError(t, kubeClient.Create(t.Context(), secret))
			} else {
				// each distinct secret loads once per selection, however many signatures use it
				require.NoError(t, err)
				require.Equal(t, "2.0.0", selected.GetVersion())
				require.NoError(t, selected.Close())
				// next selection must see a rotated key, even on the same client
				secret.Data["first"] = otherPublicKey
				require.NoError(t, kubeClient.Update(t.Context(), secret))
			}
			require.Equal(t, tt.gets, kubeClient.gets)

			selected, err = ocmClient.GetLatestValidComponentVersion(t.Context(), octx, obj)
			if tt.missingSecret {
				require.NoError(t, err)
				require.Equal(t, "2.0.0", selected.GetVersion())
				require.NoError(t, selected.Close())
			} else {
				// rotated key no longer matches; the selection fails rather than downgrading
				require.ErrorIs(t, err, ErrVerificationFailed)
				require.Nil(t, selected)
			}
			require.Equal(t, 2*tt.gets, kubeClient.gets)
		})
	}
}

// directKeyLoader loads the verification keys straight from the object, with no
// cross-candidate caching. Production selection wraps this in sync.OnceValues.
func directKeyLoader(c *Client, ctx context.Context, obj *v1alpha1.ComponentVersion) func() (map[v1alpha1.Signature][]byte, error) {
	return func() (map[v1alpha1.Signature][]byte, error) {
		return c.signaturePublicKeys(ctx, obj)
	}
}

func TestClient_VerifyComponentChecksAllSignatureNamesBeforeKeys(t *testing.T) {
	for _, tt := range []struct {
		name          string
		available     []string
		required      []string
		expectedError string
		missing       bool
		keyLookups    int
	}{
		{
			name:          "missing only required signature",
			available:     []string{"development"},
			required:      []string{"production"},
			expectedError: "signature with name 'production' not found in the list of provided ocm signatures",
			missing:       true,
		},
		{
			name:          "missing later signature prevents verification of earlier one",
			available:     []string{"development"},
			required:      []string{"development", "production"},
			expectedError: "signature with name 'production' not found in the list of provided ocm signatures",
			missing:       true,
		},
		{
			name:          "present signature still requires verification",
			available:     []string{"production"},
			required:      []string{"production"},
			expectedError: "failed to get public key for verification: unexpected public key lookup",
			keyLookups:    1,
		},
		{
			name: "no required signatures",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			octx := fakeocm.NewFakeOCMContext()
			component := &fakeocm.Component{Name: "example.com/component", Version: "1.0.0"}
			require.NoError(t, octx.AddComponent(component))
			for _, name := range tt.available {
				component.ComponentDescriptor.Signatures = append(component.ComponentDescriptor.Signatures, ocmmetav1.Signature{Name: name})
			}
			obj := &v1alpha1.ComponentVersion{
				Spec: v1alpha1.ComponentVersionSpec{
					Component:  component.Name,
					Repository: v1alpha1.Repository{URL: "localhost"},
				},
			}
			for _, name := range tt.required {
				obj.Spec.Verify = append(obj.Spec.Verify, v1alpha1.Signature{
					Name: name,
					PublicKey: v1alpha1.PublicKey{
						SecretRef: &corev1.LocalObjectReference{Name: "signing-key"},
					},
				})
			}
			kubeClient := &signatureKeyLookupClient{}
			client := NewClient(kubeClient, nil)
			err := client.verifyComponent(t.Context(), obj, component, directKeyLoader(client, t.Context(), obj))
			if tt.expectedError == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.expectedError)
				if tt.missing {
					require.ErrorIs(t, err, errRequiredSignatureMissing)
				}
			}
			require.Equal(t, tt.keyLookups, kubeClient.gets)
		})
	}
}

func TestClient_VerifyComponent(t *testing.T) {
	publicKey1, err := os.ReadFile(filepath.Join("testdata", "public1_key.pem"))
	require.NoError(t, err)
	privateKey, err := os.ReadFile(filepath.Join("testdata", "private_key.pem"))
	require.NoError(t, err)

	secretName := "sign-secret"
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: "default",
		},
		Data: map[string][]byte{
			Signature:   publicKey1,
			"unrelated": []byte("unrelated key"),
		},
	}
	fakeKubeClient := env.FakeKubeClient(WithObjects(secret))
	cache := &fakes.FakeCache{}
	ocmClient := NewClient(fakeKubeClient, cache)
	component := "ocm.software/ocm-demo-index"

	octx := fakeocm.NewFakeOCMContext()

	c := &fakeocm.Component{
		Name:    component,
		Version: "v0.0.1",
		Sign: &fakeocm.Sign{
			Name:    Signature,
			PrivKey: privateKey,
			PubKey:  publicKey1,
			Digest:  "3d879ecdea45acb7f8d85b89fd653288d84af4476eac4141822142ec59c13745",
		},
	}
	require.NoError(t, octx.AddComponent(c))
	unrelatedSignature := c.ComponentDescriptor.Signatures[0]
	unrelatedSignature.Name = "unrelated"
	unrelatedSignature.Digest.Value = strings.Repeat("0", len(unrelatedSignature.Digest.Value))
	c.ComponentDescriptor.Signatures = append(ocmmetav1.Signatures{unrelatedSignature}, c.ComponentDescriptor.Signatures...)

	cv := &v1alpha1.ComponentVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-name",
			Namespace: "default",
		},
		Spec: v1alpha1.ComponentVersionSpec{
			Component: component,
			Version: v1alpha1.Version{
				Semver: "v0.0.1",
			},
			Repository: v1alpha1.Repository{
				URL: "localhost",
			},
			Verify: []v1alpha1.Signature{
				{
					Name: Signature,
					PublicKey: v1alpha1.PublicKey{
						SecretRef: &corev1.LocalObjectReference{
							Name: secretName,
						},
					},
				},
			},
		},
	}

	err = ocmClient.verifyComponent(t.Context(), cv, c, directKeyLoader(ocmClient, t.Context(), cv))
	require.NoError(t, err)
}

func TestClient_VerifyComponentWithValueKey(t *testing.T) {
	publicKey1, err := os.ReadFile(filepath.Join("testdata", "public1_key.pem"))
	require.NoError(t, err)
	privateKey, err := os.ReadFile(filepath.Join("testdata", "private_key.pem"))
	require.NoError(t, err)

	fakeKubeClient := env.FakeKubeClient()
	cache := &fakes.FakeCache{}
	ocmClient := NewClient(fakeKubeClient, cache)
	component := "ocm.software/ocm-demo-index"

	octx := fakeocm.NewFakeOCMContext()

	c := &fakeocm.Component{
		Name:    component,
		Version: "v0.0.1",
		Sign: &fakeocm.Sign{
			Name:    Signature,
			PrivKey: privateKey,
			PubKey:  publicKey1,
			Digest:  "3d879ecdea45acb7f8d85b89fd653288d84af4476eac4141822142ec59c13745",
		},
	}
	require.NoError(t, octx.AddComponent(c))
	//var buffer []byte
	pubKey := base64.StdEncoding.EncodeToString(publicKey1)
	cv := &v1alpha1.ComponentVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-name",
			Namespace: "default",
		},
		Spec: v1alpha1.ComponentVersionSpec{
			Component: component,
			Version: v1alpha1.Version{
				Semver: "v0.0.1",
			},
			Repository: v1alpha1.Repository{
				URL: "localhost",
			},
			Verify: []v1alpha1.Signature{
				{
					Name: Signature,
					PublicKey: v1alpha1.PublicKey{
						Value: pubKey,
					},
				},
			},
		},
	}

	err = ocmClient.verifyComponent(t.Context(), cv, c, directKeyLoader(ocmClient, t.Context(), cv))
	require.NoError(t, err)
}

func TestClient_VerifyComponentWithValueKeyFailsIfValueIsEmpty(t *testing.T) {
	publicKey1, err := os.ReadFile(filepath.Join("testdata", "public1_key.pem"))
	require.NoError(t, err)
	privateKey, err := os.ReadFile(filepath.Join("testdata", "private_key.pem"))
	require.NoError(t, err)

	fakeKubeClient := env.FakeKubeClient()
	cache := &fakes.FakeCache{}
	ocmClient := NewClient(fakeKubeClient, cache)
	component := "ocm.software/ocm-demo-index"

	octx := fakeocm.NewFakeOCMContext()

	c := &fakeocm.Component{
		Name:    component,
		Version: "v0.0.1",
		Sign: &fakeocm.Sign{
			Name:    Signature,
			PrivKey: privateKey,
			PubKey:  publicKey1,
			Digest:  "3d879ecdea45acb7f8d85b89fd653288d84af4476eac4141822142ec59c13745",
		},
	}
	require.NoError(t, octx.AddComponent(c))
	cv := &v1alpha1.ComponentVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-name",
			Namespace: "default",
		},
		Spec: v1alpha1.ComponentVersionSpec{
			Component: component,
			Version: v1alpha1.Version{
				Semver: "v0.0.1",
			},
			Repository: v1alpha1.Repository{
				URL: "localhost",
			},
			Verify: []v1alpha1.Signature{
				{
					Name: Signature,
					PublicKey: v1alpha1.PublicKey{
						Value: "",
					},
				},
			},
		},
	}

	err = ocmClient.verifyComponent(t.Context(), cv, c, directKeyLoader(ocmClient, t.Context(), cv))
	assert.EqualError(t, err, "kubernetes secret reference not provided")
}

func TestClient_VerifyComponentDifferentPublicKey(t *testing.T) {
	publicKey2, err := os.ReadFile(filepath.Join("testdata", "public2_key.pem"))
	require.NoError(t, err)
	privateKey, err := os.ReadFile(filepath.Join("testdata", "private_key.pem"))
	require.NoError(t, err)

	secretName := "sign-secret"
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: "default",
		},
		Data: map[string][]byte{
			Signature: publicKey2,
		},
	}
	fakeKubeClient := env.FakeKubeClient(WithObjects(secret))
	cache := &fakes.FakeCache{}
	ocmClient := NewClient(fakeKubeClient, cache)
	component := "ocm.software/ocm-demo-index"

	octx := fakeocm.NewFakeOCMContext()

	c := &fakeocm.Component{
		Name:    component,
		Version: "v0.0.1",
		Sign: &fakeocm.Sign{
			Name:    Signature,
			PrivKey: privateKey,
			PubKey:  publicKey2,
			Digest:  "3d879ecdea45acb7f8d85b89fd653288d84af4476eac4141822142ec59c13745",
		},
	}
	require.NoError(t, octx.AddComponent(c))

	cv := &v1alpha1.ComponentVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-name",
			Namespace: "default",
		},
		Spec: v1alpha1.ComponentVersionSpec{
			Component: component,
			Version: v1alpha1.Version{
				Semver: "v0.0.1",
			},
			Repository: v1alpha1.Repository{
				URL: "localhost",
			},
			Verify: []v1alpha1.Signature{
				{
					Name: Signature,
					PublicKey: v1alpha1.PublicKey{
						SecretRef: &corev1.LocalObjectReference{
							Name: secretName,
						},
					},
				},
			},
		},
	}

	err = ocmClient.verifyComponent(t.Context(), cv, c, directKeyLoader(ocmClient, t.Context(), cv))
	require.Error(t, err)
}

func TestClient_GetResourceUsesComponentDescriptorVersionAsDefault(t *testing.T) {
	component := "ocm.software/ocm-demo-index"
	resource := "remote-controller-demo"
	componentVersion := "v1.2.3"
	reconciledVersion := "v0.9.9"
	data := "testdata"

	octx := fakeocm.NewFakeOCMContext()

	// Create component with actual component version, not reconciled version
	comp := &fakeocm.Component{
		Name:    component,
		Version: reconciledVersion,
	}
	res := &fakeocm.Resource[*ocm.ResourceMeta]{
		Name:      resource,
		Version:   componentVersion,
		Data:      []byte(data),
		Component: comp,
		Kind:      "localBlob",
		Type:      "ociBlob",
	}
	comp.Resources = append(comp.Resources, res)

	_ = octx.AddComponent(comp)

	cd := &v1alpha1.ComponentDescriptor{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "github.com-open-component-model-ocm-demo-index-v1.2.3-12345",
		},
		Spec: v1alpha1.ComponentDescriptorSpec{
			Version: componentVersion,
		},
	}

	fakeKubeClient := env.FakeKubeClient(WithObjects(cd))
	cache := &fakes.FakeCache{}
	cache.IsCachedReturns(false, nil)
	cache.FetchDataByDigestReturns(io.NopCloser(strings.NewReader("mockdata")), nil)
	cache.PushDataReturns("sha256:8fa155245ea8d3f2ea3add7d090d42dfb0e22799018fded6aae24f0c1a1c3f38", nil)

	ocmClient := NewClient(fakeKubeClient, cache)

	cv := &v1alpha1.ComponentVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-name",
			Namespace: "default",
		},
		Spec: v1alpha1.ComponentVersionSpec{
			Component: component,
			Version: v1alpha1.Version{
				Semver: componentVersion,
			},
			Repository: v1alpha1.Repository{
				URL: "localhost",
			},
		},
		Status: v1alpha1.ComponentVersionStatus{
			ReconciledVersion: reconciledVersion,
			ComponentDescriptor: v1alpha1.Reference{
				Name:    component,
				Version: componentVersion,
				ComponentDescriptorRef: meta.NamespacedObjectReference{
					Name:      "github.com-open-component-model-ocm-demo-index-v1.2.3-12345",
					Namespace: "default",
				},
			},
		},
	}

	// Test resource reference without explicit version - should use cd.Spec.Version as default
	resourceRef := &v1alpha1.ResourceReference{
		ElementMeta: v1alpha1.ElementMeta{
			Name: "remote-controller-demo",
		},
	}

	reader, digest, _, err := ocmClient.GetResource(context.Background(), octx, cv, resourceRef)
	assert.NoError(t, err)
	content, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, "mockdata", string(content))
	assert.Equal(t, "sha256:8fa155245ea8d3f2ea3add7d090d42dfb0e22799018fded6aae24f0c1a1c3f38", digest)

	// This assertion verifies that the resource version used is the component descriptor version
	// (v1.2.3) rather than the reconciled version (v0.9.9)
	args := cache.PushDataCallingArgumentsOnCall(0)
	assert.Equal(t, data, args.Content)
	assert.Equal(t, componentVersion, args.Version, "resource version should default to component descriptor version (v1.2.3), not reconciled version (v0.9.9)")
}

func TestResourceLookupWithVersionAndExtraIdentity(t *testing.T) {
	// Create Two resources, identical in name but differs in extraIdentity.
	// It should find the right resource.
	component := "ocm.software/ocm-demo-index"
	resource := "remote-controller-demo"
	resourceVersion := "v0.0.1"
	data1 := "testdata1"
	data2 := "testdata2"

	octx := fakeocm.NewFakeOCMContext()

	comp := &fakeocm.Component{
		Name:    component,
		Version: "v0.0.1",
	}
	res1 := &fakeocm.Resource[*ocm.ResourceMeta]{
		ExtraIdentity: map[string]string{
			"type": "chart",
		},
		Name:      resource,
		Version:   resourceVersion,
		Data:      []byte(data1),
		Component: comp,
		Kind:      "localBlob",
		Type:      "ociBlob",
	}
	res2 := &fakeocm.Resource[*ocm.ResourceMeta]{
		ExtraIdentity: map[string]string{
			"type": "config",
		},
		Name:      resource,
		Version:   resourceVersion,
		Data:      []byte(data2),
		Component: comp,
		Kind:      "localBlob",
		Type:      "ociBlob",
	}
	comp.Resources = append(comp.Resources, res1, res2)

	_ = octx.AddComponent(comp)

	cd := &v1alpha1.ComponentDescriptor{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "github.com-open-component-model-ocm-demo-index-v0.0.1-12345",
		},
		Spec: v1alpha1.ComponentDescriptorSpec{
			Version: "v0.0.1",
		},
	}

	fakeKubeClient := env.FakeKubeClient(WithObjects(cd))
	cache := &fakes.FakeCache{}
	cache.IsCachedReturns(false, nil)
	cache.FetchDataByDigestReturns(io.NopCloser(strings.NewReader("mockdata")), nil)
	cache.PushDataReturns("sha256:8fa155245ea8d3f2ea3add7d090d42dfb0e22799018fded6aae24f0c1a1c3f38", nil)

	ocmClient := NewClient(fakeKubeClient, cache)

	cv := &v1alpha1.ComponentVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-name",
			Namespace: "default",
		},
		Spec: v1alpha1.ComponentVersionSpec{
			Component: component,
			Version: v1alpha1.Version{
				Semver: "v0.0.1",
			},
			Repository: v1alpha1.Repository{
				URL: "localhost",
			},
		},
		Status: v1alpha1.ComponentVersionStatus{
			ReconciledVersion: "v0.0.1",
			ComponentDescriptor: v1alpha1.Reference{
				Name:    component,
				Version: "v0.0.1",
				ComponentDescriptorRef: meta.NamespacedObjectReference{
					Name:      "github.com-open-component-model-ocm-demo-index-v0.0.1-12345",
					Namespace: "default",
				},
			},
		},
	}

	resourceRef := &v1alpha1.ResourceReference{
		ElementMeta: v1alpha1.ElementMeta{
			Name:    "remote-controller-demo",
			Version: "v0.0.1",
			ExtraIdentity: map[string]string{
				"type": "config",
			},
		},
	}

	reader, digest, _, err := ocmClient.GetResource(context.Background(), octx, cv, resourceRef)
	require.NoError(t, err)
	content, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, "mockdata", string(content))
	require.Equal(t, "sha256:8fa155245ea8d3f2ea3add7d090d42dfb0e22799018fded6aae24f0c1a1c3f38", digest)

	// verify that the cache has been called with the right resource data to cache.
	args := cache.PushDataCallingArgumentsOnCall(0)

	require.Equal(t, data2, args.Content)

	require.Equal(t, "sha-17579917197306559277", args.Name, "pushed name did not match constructed name from identity of the resource")
	require.Equal(t, resourceRef.Version, args.Version)
}

type verificationComponent struct {
	ocm.ComponentVersionAccess
	resourcePasses int
	closed         int
	repository     ocm.Repository
}

func (c *verificationComponent) Repository() ocm.Repository {
	if c.repository != nil {
		return c.repository
	}
	return c.ComponentVersionAccess.Repository()
}

func (c *verificationComponent) GetResources() []ocm.ResourceAccess {
	c.resourcePasses++
	return c.ComponentVersionAccess.GetResources()
}

func (c *verificationComponent) Close() error {
	c.closed++
	return c.ComponentVersionAccess.Close()
}

type verificationRepository struct {
	ocm.Repository
	components map[string]*verificationComponent
	lookups    []string
}

func (r *verificationRepository) LookupComponentVersion(name, version string) (ocm.ComponentVersionAccess, error) {
	r.lookups = append(r.lookups, version)
	return r.components[version], nil
}

type verificationContext struct {
	ocm.Context
	repository ocm.Repository
}

func (c *verificationContext) RepositoryForSpec(ocm.RepositorySpec, ...credentials.CredentialsSource) (ocm.Repository, error) {
	return c.repository, nil
}

func TestClient_VerifyComponentSharesDigestWork(t *testing.T) {
	publicKey, err := os.ReadFile(filepath.Join("testdata", "public1_key.pem"))
	require.NoError(t, err)
	otherPublicKey, err := os.ReadFile(filepath.Join("testdata", "public2_key.pem"))
	require.NoError(t, err)
	privateKey, err := os.ReadFile(filepath.Join("testdata", "private_key.pem"))
	require.NoError(t, err)

	for _, tt := range []struct {
		name           string
		secondKey      []byte
		secondHash     string
		duplicateName  bool
		wantError      bool
		resourcePasses int
	}{
		{name: "shared digest", secondKey: publicKey, resourcePasses: 1},
		{name: "later signature must verify", secondKey: otherPublicKey, wantError: true, resourcePasses: 1},
		{name: "different digest algorithms", secondKey: publicKey, secondHash: "SHA-512", resourcePasses: 2},
		{name: "different digest algorithm must verify", secondKey: otherPublicKey, secondHash: "SHA-512", wantError: true, resourcePasses: 2},
		{name: "duplicate names cannot replace an earlier key", secondKey: publicKey, duplicateName: true, wantError: true, resourcePasses: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			octx := fakeocm.NewFakeOCMContext()
			component := &fakeocm.Component{
				Name: "example.com/component", Version: "1.0.0",
				Sign: &fakeocm.Sign{Name: "first", PrivKey: privateKey, PubKey: publicKey},
			}
			require.NoError(t, octx.AddComponent(component))
			second := component.ComponentDescriptor.Signatures[0]
			second.Name = "second"
			if tt.secondHash != "" {
				opts := signing.NewOptions(
					signing.SignByAlgo("RSASSA-PKCS1-V1_5", second.Name),
					signing.HashByAlgo(tt.secondHash),
					signing.PrivateKey(second.Name, privateKey),
					signing.Resolver(resolvers.NewCompoundResolver(component.Repository())),
					signing.Update(),
				)
				_, err := signing.ApplyWithContext(t.Context(), nil, component, opts)
				require.NoError(t, err)
			} else {
				component.ComponentDescriptor.Signatures = append(component.ComponentDescriptor.Signatures, second)
			}
			obj := &v1alpha1.ComponentVersion{
				Spec: v1alpha1.ComponentVersionSpec{
					Verify: []v1alpha1.Signature{
						{Name: "first", PublicKey: v1alpha1.PublicKey{Value: base64.StdEncoding.EncodeToString(publicKey)}},
						{Name: "second", PublicKey: v1alpha1.PublicKey{Value: base64.StdEncoding.EncodeToString(tt.secondKey)}},
					},
				},
			}
			if tt.duplicateName {
				obj.Spec.Verify[0].PublicKey.Value = base64.StdEncoding.EncodeToString(otherPublicKey)
				obj.Spec.Verify[1].Name = "first"
			}
			counted := &verificationComponent{ComponentVersionAccess: component}
			client := NewClient(env.FakeKubeClient(), nil)
			err = client.verifyComponent(t.Context(), obj, counted, directKeyLoader(client, t.Context(), obj))
			if tt.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.resourcePasses, counted.resourcePasses)
			require.Zero(t, counted.closed)
		})
	}
}

func TestClient_GetLatestValidComponentVersionReusesVerifiedComponent(t *testing.T) {
	octx := fakeocm.NewFakeOCMContext()
	repo, err := octx.RepositoryForSpec(nil)
	require.NoError(t, err)
	countedRepo := &verificationRepository{Repository: repo, components: make(map[string]*verificationComponent)}
	for _, version := range []string{"1.0.0", "2.0.0", "3.0.0"} {
		component := &fakeocm.Component{Name: "example.com/component", Version: version}
		require.NoError(t, octx.AddComponent(component))
		if version == "2.0.0" {
			privateKey, err := os.ReadFile(filepath.Join("testdata", "private_key.pem"))
			require.NoError(t, err)
			opts := signing.NewOptions(signing.SignByAlgo("RSASSA-PKCS1-V1_5", "production"), signing.PrivateKey("production", privateKey), signing.Update())
			_, err = signing.ApplyWithContext(t.Context(), nil, component, opts)
			require.NoError(t, err)
		}
		countedRepo.components[version] = &verificationComponent{ComponentVersionAccess: component}
	}
	publicKey, err := os.ReadFile(filepath.Join("testdata", "public1_key.pem"))
	require.NoError(t, err)
	obj := &v1alpha1.ComponentVersion{
		Spec: v1alpha1.ComponentVersionSpec{
			Component: "example.com/component", Repository: v1alpha1.Repository{URL: "localhost"},
			Version: v1alpha1.Version{Semver: ">=1.0.0"},
			Verify:  []v1alpha1.Signature{{Name: "production", PublicKey: v1alpha1.PublicKey{Value: base64.StdEncoding.EncodeToString(publicKey)}}},
		},
	}
	ctx := &verificationContext{Context: octx, repository: countedRepo}
	selected, err := NewClient(env.FakeKubeClient(), nil).GetLatestValidComponentVersion(t.Context(), ctx, obj)
	require.NoError(t, err)
	require.Same(t, countedRepo.components["2.0.0"], selected)
	require.Equal(t, []string{"3.0.0", "2.0.0"}, countedRepo.lookups)
	require.Equal(t, 1, countedRepo.components["3.0.0"].closed)
	require.Zero(t, countedRepo.components["2.0.0"].closed)
	require.Zero(t, countedRepo.components["1.0.0"].closed)
	require.Equal(t, 1, countedRepo.components["2.0.0"].resourcePasses)
	require.NoError(t, selected.Close())
	require.Equal(t, 1, countedRepo.components["2.0.0"].closed)
}

func TestClient_VerifyComponentSharesReferenceLookups(t *testing.T) {
	publicKey, err := os.ReadFile(filepath.Join("testdata", "public1_key.pem"))
	require.NoError(t, err)
	privateKey, err := os.ReadFile(filepath.Join("testdata", "private_key.pem"))
	require.NoError(t, err)
	octx := fakeocm.NewFakeOCMContext()
	child := &fakeocm.Component{Name: "example.com/child", Version: "1.0.0"}
	root := &fakeocm.Component{Name: "example.com/root", Version: "2.0.0"}
	require.NoError(t, octx.AddComponent(child))
	require.NoError(t, octx.AddComponent(root))
	root.ComponentDescriptor.References = append(root.ComponentDescriptor.References, compdesc.Reference{
		ElementMeta: compdesc.ElementMeta{Name: "child", Version: child.Version}, ComponentName: child.Name,
	})
	opts := signing.NewOptions(
		signing.SignByAlgo("RSASSA-PKCS1-V1_5", "first"), signing.PrivateKey("first", privateKey),
		signing.Resolver(resolvers.NewCompoundResolver(root.Repository())), signing.Update(),
	)
	_, err = signing.ApplyWithContext(t.Context(), nil, root, opts)
	require.NoError(t, err)
	second := root.ComponentDescriptor.Signatures[0]
	second.Name = "second"
	root.ComponentDescriptor.Signatures = append(root.ComponentDescriptor.Signatures, second)
	countedRepo := &verificationRepository{
		Repository: root.Repository(),
		components: map[string]*verificationComponent{child.Version: {ComponentVersionAccess: child}},
	}
	counted := &verificationComponent{ComponentVersionAccess: root, repository: countedRepo}
	obj := &v1alpha1.ComponentVersion{Spec: v1alpha1.ComponentVersionSpec{}}
	for _, name := range []string{"first", "second"} {
		obj.Spec.Verify = append(obj.Spec.Verify, v1alpha1.Signature{
			Name: name, PublicKey: v1alpha1.PublicKey{Value: base64.StdEncoding.EncodeToString(publicKey)},
		})
	}
	client := NewClient(env.FakeKubeClient(), nil)
	err = client.verifyComponent(t.Context(), obj, counted, directKeyLoader(client, t.Context(), obj))
	require.NoError(t, err)
	require.Equal(t, []string{child.Version}, countedRepo.lookups)
	require.Equal(t, 1, counted.resourcePasses)
	require.Equal(t, 1, countedRepo.components[child.Version].resourcePasses)
	require.Equal(t, 1, countedRepo.components[child.Version].closed)
}

func TestClient_GetLatestValidComponentVersionClosesRejectedComponents(t *testing.T) {
	octx := fakeocm.NewFakeOCMContext()
	repo, err := octx.RepositoryForSpec(nil)
	require.NoError(t, err)
	countedRepo := &verificationRepository{Repository: repo, components: make(map[string]*verificationComponent)}
	for _, version := range []string{"1.0.0", "2.0.0"} {
		component := &fakeocm.Component{Name: "example.com/component", Version: version}
		require.NoError(t, octx.AddComponent(component))
		countedRepo.components[version] = &verificationComponent{ComponentVersionAccess: component}
	}
	obj := &v1alpha1.ComponentVersion{
		Spec: v1alpha1.ComponentVersionSpec{
			Component: "example.com/component", Repository: v1alpha1.Repository{URL: "localhost"},
			Version: v1alpha1.Version{Semver: ">=1.0.0"},
			Verify:  []v1alpha1.Signature{{Name: "production"}},
		},
	}
	ctx := &verificationContext{Context: octx, repository: countedRepo}
	selected, err := NewClient(env.FakeKubeClient(), nil).GetLatestValidComponentVersion(t.Context(), ctx, obj)
	// matched the constraint but failed verification; must be distinct from no matching versions
	require.ErrorIs(t, err, ErrVerificationFailed)
	require.ErrorContains(t, err, "signature with name 'production' not found")
	require.Nil(t, selected)
	require.Equal(t, []string{"2.0.0", "1.0.0"}, countedRepo.lookups)
	for _, component := range countedRepo.components {
		require.Equal(t, 1, component.closed)
		require.Zero(t, component.resourcePasses)
	}
}

func TestClient_GetLatestValidComponentVersionNoMatchingConstraint(t *testing.T) {
	octx := fakeocm.NewFakeOCMContext()
	for _, version := range []string{"1.0.0", "2.0.0"} {
		require.NoError(t, octx.AddComponent(&fakeocm.Component{Name: "example.com/component", Version: version}))
	}
	obj := &v1alpha1.ComponentVersion{
		Spec: v1alpha1.ComponentVersionSpec{
			Component:  "example.com/component",
			Repository: v1alpha1.Repository{URL: "localhost"},
			Version:    v1alpha1.Version{Semver: ">=3.0.0"},
		},
	}
	selected, err := NewClient(env.FakeKubeClient(), nil).GetLatestValidComponentVersion(t.Context(), octx, obj)
	// no version satisfies the constraint; not a verification failure
	require.ErrorContains(t, err, "no matching versions found")
	require.NotErrorIs(t, err, ErrVerificationFailed)
	require.Nil(t, selected)
}
