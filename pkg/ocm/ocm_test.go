// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package ocm

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/distribution/distribution/v3/registry/storage/driver/inmemory"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/open-component-model/ocm/pkg/contexts/credentials/cpi"
	"github.com/open-component-model/ocm/pkg/contexts/oci/identity"
	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/ocm.software/v3alpha1"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/pkg/oci"
)

func TestClient_GetResource(t *testing.T) {
	component := "github.com/skarlso/ocm-demo-index"
	resource := "remote-controller-demo"
	resourceVersion := "v0.0.1"
	data := "testdata"

	res := Resource{
		Name:    resource,
		Version: resourceVersion,
		Data:    data,
	}

	err := env.AddComponentVersionToRepository(Component{
		Name:    component,
		Version: "v0.0.1",
	}, res)
	require.NoError(t, err)

	cd := &v1alpha1.ComponentDescriptor{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "github.com-skarlso-ocm-demo-index-v0.0.1-12345",
		},
		Spec: v1alpha1.ComponentDescriptorSpec{
			Version: "v0.0.1",
		},
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "registry-certs",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"caFile":   []byte("file"),
			"certFile": []byte("file"),
			"keyFile":  []byte("file"),
		},
		Type: "Opaque",
	}

	fakeKubeClient := env.FakeKubeClient(WithObjects(secret, cd))
	cache := oci.NewClient(strings.TrimPrefix(env.repositoryURL, "http://"),
		oci.WithClient(fakeKubeClient),
		oci.WithNamespace("default"),
		oci.WithCertificateSecret("registry-certs"),
	)
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
				URL: env.repositoryURL,
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
		ElementMeta: v3alpha1.ElementMeta{
			Name:    "remote-controller-demo",
			Version: "v0.0.1",
		},
	}

	reader, digest, err := ocmClient.GetResource(context.Background(), ocm.New(), cv, resourceRef)
	assert.NoError(t, err)
	content, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, data, string(content))
	assert.Equal(t, "sha256:8fa155245ea8d3f2ea3add7d090d42dfb0e22799018fded6aae24f0c1a1c3f38", digest)
}

func TestClient_GetComponentVersion(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "registry-certs",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"caFile":   []byte("file"),
			"certFile": []byte("file"),
			"keyFile":  []byte("file"),
		},
		Type: "Opaque",
	}

	fakeKubeClient := env.FakeKubeClient(WithObjects(secret))
	cache := oci.NewClient(strings.TrimPrefix(env.repositoryURL, "http://"), oci.WithClient(fakeKubeClient))
	ocmClient := NewClient(fakeKubeClient, cache)
	component := "github.com/skarlso/ocm-demo-index"

	err := env.AddComponentVersionToRepository(Component{
		Name:    component,
		Version: "v0.0.1",
	})
	require.NoError(t, err)

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
				URL: env.repositoryURL,
			},
		},
		Status: v1alpha1.ComponentVersionStatus{
			ReconciledVersion: "v0.0.1",
		},
	}

	cva, err := ocmClient.GetComponentVersion(context.Background(), ocm.New(), cv, component, "v0.0.1")
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
				URL: env.repositoryURL,
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

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "registry-certs",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"caFile":   []byte("file"),
			"certFile": []byte("file"),
			"keyFile":  []byte("file"),
		},
		Type: "Opaque",
	}
	trimmedURL := strings.TrimPrefix(env.repositoryURL, "http://")
	fakeKubeClient := env.FakeKubeClient(WithObjects(secret, cs, testSecret))
	cache := oci.NewClient(trimmedURL, oci.WithClient(fakeKubeClient))
	ocmClient := NewClient(fakeKubeClient, cache)

	err := env.AddComponentVersionToRepository(Component{
		Name:    component,
		Version: "v0.0.1",
	})
	require.NoError(t, err)

	octx, err := ocmClient.CreateAuthenticatedOCMContext(context.Background(), cs)
	require.NoError(t, err)

	id := cpi.ConsumerIdentity{
		cpi.ID_TYPE:            identity.CONSUMER_TYPE,
		identity.ID_HOSTNAME:   trimmedURL,
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
				URL: env.repositoryURL,
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
    "ghcr.io": {
      "username": "skarlso",
      "password": "password",
      "auth": "c2thcmxzbzpwYXNzd29yZAo="
    }
  }
}`),
		},
		Type: corev1.SecretTypeDockerConfigJson,
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "registry-certs",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"caFile":   []byte("file"),
			"certFile": []byte("file"),
			"keyFile":  []byte("file"),
		},
		Type: "Opaque",
	}

	fakeKubeClient := env.FakeKubeClient(WithObjects(secret, cs, serviceAccount, testSecret))
	trimmedURL := strings.TrimPrefix(env.repositoryURL, "http://")
	cache := oci.NewClient(trimmedURL, oci.WithClient(fakeKubeClient))
	ocmClient := NewClient(fakeKubeClient, cache)
	component := "github.com/skarlso/ocm-demo-index"

	err := env.AddComponentVersionToRepository(Component{
		Name:    component,
		Version: "v0.0.1",
	})
	require.NoError(t, err)

	octx, err := ocmClient.CreateAuthenticatedOCMContext(context.Background(), cs)
	require.NoError(t, err)

	id := cpi.ConsumerIdentity{
		cpi.ID_TYPE:            identity.CONSUMER_TYPE,
		identity.ID_HOSTNAME:   "ghcr.io",
		identity.ID_PATHPREFIX: "skarlso",
	}
	creds, err := octx.CredentialsContext().GetCredentialsForConsumer(id)
	require.NoError(t, err)
	consumer, err := creds.Credentials(nil)
	require.NoError(t, err)

	assert.Equal(t, "password", consumer.Properties()["password"])
	assert.Equal(t, "skarlso", consumer.Properties()["username"])
	assert.Equal(t, "ghcr.io", consumer.Properties()["serverAddress"])
}

func TestClient_GetLatestValidComponentVersion(t *testing.T) {
	testCases := []struct {
		name             string
		componentVersion func(name string) *v1alpha1.ComponentVersion
		setupComponents  func(name string) error
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
							URL: env.repositoryURL,
						},
					},
				}
			},
			setupComponents: func(name string) error {
				// v0.0.1 should not be chosen.
				for _, v := range []string{"v0.0.1", "v0.0.5"} {
					if err := env.AddComponentVersionToRepository(Component{
						Name:    name,
						Version: v,
					}); err != nil {
						return err
					}
				}
				return nil
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
							URL: env.repositoryURL,
						},
					},
				}
			},
			setupComponents: func(name string) error {
				for _, v := range []string{"v0.0.1", "v0.0.2", "v0.0.3"} {
					if err := env.AddComponentVersionToRepository(Component{
						Name:    name,
						Version: v,
					}); err != nil {
						return err
					}
				}
				return nil
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
							URL: env.repositoryURL,
						},
					},
				}
			},
			setupComponents: func(name string) error {
				for _, v := range []string{"v0.0.1", "v0.0.2", "v0.0.3"} {
					if err := env.AddComponentVersionToRepository(Component{
						Name:    name,
						Version: v,
					}); err != nil {
						return err
					}
				}
				return nil
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
							URL: env.repositoryURL,
						},
					},
				}
			},
			setupComponents: func(name string) error {
				for _, v := range []string{"v0.0.1", "v0.0.2"} {
					if err := env.AddComponentVersionToRepository(Component{
						Name:    name,
						Version: v,
					}); err != nil {
						return err
					}
				}
				return nil
			},
			expectedVersion: "v0.0.1",
		},
	}
	key, _ := os.ReadFile(filepath.Join("testdata", "key.pem"))
	cert, _ := os.ReadFile(filepath.Join("testdata", "cert.pem"))
	rootCA, _ := os.ReadFile(filepath.Join("testdata", "ca.pem"))
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "registry-certs",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"caFile":   rootCA,
			"certFile": cert,
			"keyFile":  key,
		},
		Type: "Opaque",
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()

			fakeKubeClient := env.FakeKubeClient(WithObjects(secret))
			cache := oci.NewClient(env.repositoryURL, oci.WithClient(fakeKubeClient), oci.WithNamespace("default"), oci.WithCertificateSecret("registry-certs"))
			ocmClient := NewClient(fakeKubeClient, cache)
			component := "github.com/skarlso/ocm-demo-index"

			err := tt.setupComponents(component)
			require.NoError(t, err)
			cv := tt.componentVersion(component)

			latest, err := ocmClient.GetLatestValidComponentVersion(context.Background(), ocm.New(), cv)
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
	certSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "registry-certs",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"caFile":   []byte("file"),
			"certFile": []byte("file"),
			"keyFile":  []byte("file"),
		},
		Type: "Opaque",
	}

	fakeKubeClient := env.FakeKubeClient(WithObjects(secret, certSecret))
	cache := oci.NewClient(strings.TrimPrefix(env.repositoryURL, "http://"), oci.WithClient(fakeKubeClient), oci.WithCertificateSecret("registry-certs"), oci.WithNamespace("default"))
	ocmClient := NewClient(fakeKubeClient, cache)
	component := "github.com/skarlso/ocm-demo-index"

	err = env.AddComponentVersionToRepository(Component{
		Name:    component,
		Version: "v0.0.1",
		Sign: &Sign{
			Name: Signature,
			Key:  privateKey,
		},
	})
	require.NoError(t, err)

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
				URL: env.repositoryURL,
			},
			Verify: []v1alpha1.Signature{
				{
					Name: Signature,
					PublicKey: v1alpha1.SecretRef{
						SecretRef: corev1.LocalObjectReference{
							Name: secretName,
						},
					},
				},
			},
		},
	}

	verified, err := ocmClient.VerifyComponent(context.Background(), ocm.New(), cv, "v0.0.1")
	assert.NoError(t, err)
	assert.True(t, verified, "verified should have been true, but it did not")
}

func TestClient_VerifyComponentDifferentPublicKey(t *testing.T) {
	publicKey2, err := os.ReadFile(filepath.Join("testdata", "public2_key.pem"))
	require.NoError(t, err)
	privateKey, err := os.ReadFile(filepath.Join("testdata", "private_key.pem"))
	require.NoError(t, err)

	secretName := "sign-secret-2"
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: "default",
		},
		Data: map[string][]byte{
			Signature: publicKey2,
		},
	}
	certSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "registry-certs",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"caFile":   []byte("file"),
			"certFile": []byte("file"),
			"keyFile":  []byte("file"),
		},
		Type: "Opaque",
	}

	fakeKubeClient := env.FakeKubeClient(WithObjects(secret, certSecret))
	cache := oci.NewClient(strings.TrimPrefix(env.repositoryURL, "http://"), oci.WithClient(fakeKubeClient), oci.WithNamespace("default"), oci.WithCertificateSecret("registry-certs"))
	ocmClient := NewClient(fakeKubeClient, cache)
	component := "github.com/skarlso/ocm-demo-index"

	err = env.AddComponentVersionToRepository(Component{
		Name:    component,
		Version: "v0.0.1",
		Sign: &Sign{
			Name: Signature,
			Key:  privateKey,
		},
	})
	require.NoError(t, err)

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
				URL: env.repositoryURL,
				SecretRef: &corev1.LocalObjectReference{
					Name: secretName,
				},
			},
			Verify: []v1alpha1.Signature{
				{
					Name: Signature,
					PublicKey: v1alpha1.SecretRef{
						SecretRef: corev1.LocalObjectReference{
							Name: secretName,
						},
					},
				},
			},
		},
	}

	verified, err := ocmClient.VerifyComponent(context.Background(), ocm.New(), cv, "v0.0.1")
	require.Error(t, err)
	assert.False(t, verified, "verified should have been false, but it did not")
}
