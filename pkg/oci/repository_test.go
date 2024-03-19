// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package oci

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	_ "github.com/distribution/distribution/v3/registry/storage/driver/inmemory"
	"github.com/google/go-containerregistry/pkg/v1/types"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/pkg/ocm"
	ocmmetav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
)

func TestRepository_Blob(t *testing.T) {
	addr := strings.TrimPrefix(testServer.URL, "http://")
	testCases := []struct {
		name     string
		blob     []byte
		expected []byte
	}{
		{
			name:     "blob",
			blob:     []byte("blob"),
			expected: []byte("blob"),
		},
		{
			name:     "empty blob",
			blob:     []byte(""),
			expected: []byte(""),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			// create a repository client
			repoName := generateRandomName("testblob")
			repo, err := NewRepository(addr + "/" + repoName)
			g.Expect(err).NotTo(HaveOccurred())

			// compute a blob
			layer := computeStreamBlob(io.NopCloser(bytes.NewBuffer(tc.blob)), string(types.OCILayer))

			// push blob to the registry
			err = repo.pushBlob(layer)
			g.Expect(err).NotTo(HaveOccurred())

			// fetch the blob from the registry
			digest, err := layer.Digest()
			g.Expect(err).NotTo(HaveOccurred())
			rc, err := repo.FetchBlob(digest.String())
			g.Expect(err).NotTo(HaveOccurred())
			b, err := io.ReadAll(rc)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(b).To(Equal(tc.expected))
		})
	}
}

func TestRepository_StreamImage(t *testing.T) {
	addr := strings.TrimPrefix(testServer.URL, "http://")
	testCases := []struct {
		name     string
		blob     []byte
		expected []byte
	}{
		{
			name:     "image",
			blob:     []byte("image"),
			expected: []byte("image"),
		},
		{
			name:     "empty image",
			blob:     []byte(""),
			expected: []byte(""),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			// create a repository client
			repoName := generateRandomName("testimage")
			repo, err := NewRepository(addr + "/" + repoName)
			g.Expect(err).NotTo(HaveOccurred())

			// push image to the registry
			blob := tc.blob
			reader := io.NopCloser(bytes.NewBuffer(blob))
			manifest, err := repo.PushStreamingImage("latest", reader, string(types.OCILayer), map[string]string{
				"org.opencontainers.artifact.created": time.Now().UTC().Format(time.RFC3339),
			})
			g.Expect(err).NotTo(HaveOccurred())
			digest := manifest.Layers[0].Digest.String()
			layer, err := repo.FetchBlob(digest)
			g.Expect(err).NotTo(HaveOccurred())
			b, err := io.ReadAll(layer)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(b).To(Equal(tc.expected))
		})
	}
}

func TestClient_FetchPush(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, v1alpha1.AddToScheme(scheme))
	assert.NoError(t, v1.AddToScheme(scheme))

	addr := strings.TrimPrefix(testServer.URL, "http://")
	testCases := []struct {
		name     string
		blob     []byte
		expected []byte
		resource v1alpha1.ResourceReference
		objects  []client.Object
		push     bool
	}{
		{
			name:     "image",
			blob:     []byte("image"),
			expected: []byte("image"),
			resource: v1alpha1.ResourceReference{
				ElementMeta: v1alpha1.ElementMeta{
					Name:    "test-resource-1",
					Version: "v0.0.1",
				},
			},
			push: true,
		},
		{
			name:     "empty image",
			blob:     []byte(""),
			expected: []byte(""),
			resource: v1alpha1.ResourceReference{
				ElementMeta: v1alpha1.ElementMeta{
					Name:    "test-resource-2",
					Version: "v0.0.2",
				},
			},
			push: true,
		},
		{
			name:     "data doesn't exist",
			blob:     []byte(""),
			expected: []byte(""),
			resource: v1alpha1.ResourceReference{
				ElementMeta: v1alpha1.ElementMeta{
					Name:    "test-resource-2",
					Version: "v0.0.3",
				},
			},
		},
	}

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ocm-registry-tls-certs",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"ca.crt":  []byte("file"),
			"tls.crt": []byte("file"),
			"tls.key": []byte("file"),
		},
		Type: "Opaque",
	}
	fakeClient := fake.NewClientBuilder().WithObjects(secret).WithScheme(scheme).Build()
	c := NewClient(addr, WithClient(fakeClient), WithCertificateSecret("ocm-registry-tls-certs"), WithNamespace("default"))

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Helper()

			g := NewWithT(t)
			obj := &v1alpha1.ComponentVersion{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-name",
					Namespace: "default",
				},
				Spec: v1alpha1.ComponentVersionSpec{
					Component: "github.com/skarlso/root",
					Version: v1alpha1.Version{
						Semver: "v0.0.1",
					},
				},
				Status: v1alpha1.ComponentVersionStatus{
					ReconciledVersion: "v0.0.1",
				},
			}
			identity := ocmmetav1.Identity{
				v1alpha1.ComponentVersionKey: obj.Status.ReconciledVersion,
				v1alpha1.ComponentNameKey:    obj.Spec.Component,
				v1alpha1.ResourceNameKey:     tc.resource.Name,
				v1alpha1.ResourceVersionKey:  tc.resource.Version,
			}
			name, err := ocm.HashIdentity(identity)
			g.Expect(err).NotTo(HaveOccurred())
			if tc.push {
				_, _, err := c.PushData(context.Background(), io.NopCloser(bytes.NewBuffer(tc.blob)), "", name, tc.resource.Version)
				g.Expect(err).NotTo(HaveOccurred())
				blob, _, _, err := c.FetchDataByIdentity(context.Background(), name, tc.resource.Version)
				g.Expect(err).NotTo(HaveOccurred())
				content, err := io.ReadAll(blob)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(content).To(Equal(tc.expected))
			} else {
				exists, err := c.IsCached(context.Background(), name, tc.resource.Version)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(exists).To(BeFalse())
			}
		})
	}
}

func TestClient_DeleteData(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, v1.AddToScheme(scheme))
	assert.NoError(t, v1alpha1.AddToScheme(scheme))

	addr := strings.TrimPrefix(testServer.URL, "http://")
	testCases := []struct {
		name     string
		blob     []byte
		expected []byte
		resource v1alpha1.ResourceReference
		objects  []client.Object
		push     bool
	}{
		{
			name:     "image",
			blob:     []byte("image"),
			expected: []byte("image"),
			resource: v1alpha1.ResourceReference{
				ElementMeta: v1alpha1.ElementMeta{
					Name:    "test-resource-1",
					Version: "v0.0.1",
				},
			},
			push: true,
		},
	}

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ocm-registry-tls-certs",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"ca.crt":  []byte("file"),
			"tls.crt": []byte("file"),
			"tls.key": []byte("file"),
		},
		Type: "Opaque",
	}
	fakeClient := fake.NewClientBuilder().WithObjects(secret).WithScheme(scheme).Build()
	c := NewClient(addr, WithClient(fakeClient), WithCertificateSecret("ocm-registry-tls-certs"), WithNamespace("default"))

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Helper()

			g := NewWithT(t)

			obj := &v1alpha1.ComponentVersion{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-name",
					Namespace: "default",
				},
				Spec: v1alpha1.ComponentVersionSpec{
					Component: "github.com/skarlso/root",
					Version: v1alpha1.Version{
						Semver: "v0.0.1",
					},
				},
				Status: v1alpha1.ComponentVersionStatus{
					ReconciledVersion: "v0.0.1",
				},
			}
			identity := ocmmetav1.Identity{
				v1alpha1.ComponentVersionKey: obj.Status.ReconciledVersion,
				v1alpha1.ComponentNameKey:    obj.Spec.Component,
				v1alpha1.ResourceNameKey:     tc.resource.Name,
				v1alpha1.ResourceVersionKey:  tc.resource.Version,
			}
			name, err := ocm.HashIdentity(identity)
			g.Expect(err).NotTo(HaveOccurred())
			_, _, err = c.PushData(context.Background(), io.NopCloser(bytes.NewBuffer(tc.blob)), "", name, tc.resource.Version)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(c.DeleteData(context.Background(), name, tc.resource.Version)).To(Succeed())
			exists, err := c.IsCached(context.Background(), name, tc.resource.Version)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(exists).To(BeFalse())
		})
	}
}
