// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package ocm

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/containers/image/v5/pkg/compression"
	_ "github.com/distribution/distribution/v3/registry/storage/driver/inmemory"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/open-component-model/ocm/pkg/contexts/credentials/cpi"
	"github.com/open-component-model/ocm/pkg/contexts/oci/identity"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/pkg/cache/fakes"
	fakeocm "github.com/open-component-model/ocm-controller/pkg/fakes"
)

func TestClient_GetResource(t *testing.T) {
	component := "github.com/skarlso/ocm-demo-index"
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
			Name:      "github.com-skarlso-ocm-demo-index-v0.0.1-12345",
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
					Name:      "github.com-skarlso-ocm-demo-index-v0.0.1-12345",
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

	assert.Equal(t, "sha-2705577397727487661", args.Name, "pushed name did not match constructed name from identity of the resource")
	assert.Equal(t, resourceRef.Version, args.Version)
}

func TestClient_GetHelmResource(t *testing.T) {
	component := "github.com/skarlso/ocm-demo-index"
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
			Name:      "github.com-skarlso-ocm-demo-index-v0.0.1-12345",
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
					Name:      "github.com-skarlso-ocm-demo-index-v0.0.1-12345",
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

	assert.Equal(t, "sha-2705577397727487661", args.Name, "pushed name did not match constructed name from identity of the resource")
	assert.Equal(t, resourceRef.Version, args.Version)
}

func TestClient_GetComponentVersion(t *testing.T) {
	component := "github.com/skarlso/ocm-demo-index"
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
			ReconciledVersion: "v0.0.1",
		},
	}

	cva, err := ocmClient.GetComponentVersion(context.Background(), octx, cv, component, "v0.0.1")
	assert.NoError(t, err)
	assert.Equal(t, cv.Spec.Component, cva.GetName())
}

func TestClient_CreateAuthenticatedOCMContextWithSecret(t *testing.T) {
	component := "github.com/skarlso/ocm-demo-index"
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
		identity.ID_PATHPREFIX: "skarlso",
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
			Component: "github.com/skarlso/ocm-demo-index",
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
      "username": "skarlso",
      "password": "password",
      "auth": "c2thcmxzbzpwYXNzd29yZAo="
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
		identity.ID_PATHPREFIX: "skarlso",
	}
	creds, err := octx.CredentialsContext().GetCredentialsForConsumer(id)
	require.NoError(t, err)
	consumer, err := creds.Credentials(nil)
	require.NoError(t, err)

	assert.Equal(t, "password", consumer.Properties()["password"])
	assert.Equal(t, "skarlso", consumer.Properties()["username"])
	assert.Equal(t, "localhost", consumer.Properties()["serverAddress"])
}

func TestClient_GetLatestValidComponentVersion(t *testing.T) {
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
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()

			fakeKubeClient := env.FakeKubeClient()
			cache := &fakes.FakeCache{}
			ocmClient := NewClient(fakeKubeClient, cache)
			octx := fakeocm.NewFakeOCMContext()
			component := "github.com/skarlso/ocm-demo-index"

			tt.setupComponents(component, octx)
			cv := tt.componentVersion(component)

			latest, err := ocmClient.GetLatestValidComponentVersion(context.Background(), octx, cv)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedVersion, latest)
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
			Signature: publicKey1,
		},
	}
	fakeKubeClient := env.FakeKubeClient(WithObjects(secret))
	cache := &fakes.FakeCache{}
	ocmClient := NewClient(fakeKubeClient, cache)
	component := "github.com/skarlso/ocm-demo-index"

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
						SecretRef: &corev1.LocalObjectReference{
							Name: secretName,
						},
					},
				},
			},
		},
	}

	verified, err := ocmClient.VerifyComponent(context.Background(), octx, cv, "v0.0.1")
	require.NoError(t, err)
	assert.True(t, verified, "verified should have been true, but it did not")
}

func TestClient_VerifyComponentWithValueKey(t *testing.T) {
	publicKey1, err := os.ReadFile(filepath.Join("testdata", "public1_key.pem"))
	require.NoError(t, err)
	privateKey, err := os.ReadFile(filepath.Join("testdata", "private_key.pem"))
	require.NoError(t, err)

	fakeKubeClient := env.FakeKubeClient()
	cache := &fakes.FakeCache{}
	ocmClient := NewClient(fakeKubeClient, cache)
	component := "github.com/skarlso/ocm-demo-index"

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

	verified, err := ocmClient.VerifyComponent(context.Background(), octx, cv, "v0.0.1")
	require.NoError(t, err)
	assert.True(t, verified, "verified should have been true, but it did not")
}

func TestClient_VerifyComponentWithValueKeyFailsIfValueIsEmpty(t *testing.T) {
	publicKey1, err := os.ReadFile(filepath.Join("testdata", "public1_key.pem"))
	require.NoError(t, err)
	privateKey, err := os.ReadFile(filepath.Join("testdata", "private_key.pem"))
	require.NoError(t, err)

	fakeKubeClient := env.FakeKubeClient()
	cache := &fakes.FakeCache{}
	ocmClient := NewClient(fakeKubeClient, cache)
	component := "github.com/skarlso/ocm-demo-index"

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

	_, err = ocmClient.VerifyComponent(context.Background(), octx, cv, "v0.0.1")
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
	component := "github.com/skarlso/ocm-demo-index"

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

	verified, err := ocmClient.VerifyComponent(context.Background(), octx, cv, "v0.0.1")
	require.Error(t, err)
	assert.False(t, verified, "verified should have been false, but it did not")
}
