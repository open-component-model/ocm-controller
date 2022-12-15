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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/pkg/cache"
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
			layer, err := computeStreamBlob(io.NopCloser(bytes.NewBuffer(tc.blob)), string(types.OCILayer))
			g.Expect(err).NotTo(HaveOccurred())

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
			layerDigest, err := repo.PushStreamingImage("latest", reader, string(types.OCILayer), map[string]string{
				"org.opencontainers.artifact.created": time.Now().UTC().Format(time.RFC3339),
			})
			g.Expect(err).NotTo(HaveOccurred())
			layer, err := repo.FetchBlob(layerDigest)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(layerDigest).To(Equal(layerDigest))
			b, err := io.ReadAll(layer)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(b).To(Equal(tc.expected))
		})
	}
}

func TestClient_FetchPush(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	assert.NoError(t, err)

	addr := strings.TrimPrefix(testServer.URL, "http://")
	testCases := []struct {
		name     string
		blob     []byte
		expected []byte
		resource v1alpha1.ResourceRef
		objects  []client.Object
		push     bool
	}{
		{
			name:     "image",
			blob:     []byte("image"),
			expected: []byte("image"),
			resource: v1alpha1.ResourceRef{
				Name:    "test-resource-1",
				Version: "v0.0.1",
			},
			push: true,
		},
		{
			name:     "empty image",
			blob:     []byte(""),
			expected: []byte(""),
			resource: v1alpha1.ResourceRef{
				Name:    "test-resource-2",
				Version: "v0.0.1",
			},
			push: true,
		},
		{
			name:     "data doesn't exist",
			blob:     []byte(""),
			expected: []byte(""),
			resource: v1alpha1.ResourceRef{
				Name:    "test-resource-2",
				Version: "v0.0.1",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			c := NewClient(addr)
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
			identity := v1alpha1.Identity{
				cache.ComponentVersionKey: obj.Status.ReconciledVersion,
				cache.ComponentNameKey:    obj.Spec.Component,
				cache.ResourceNameKey:     tc.resource.Name,
				cache.ResourceVersionKey:  tc.resource.Version,
			}
			if tc.push {
				_, err := c.PushData(context.Background(), io.NopCloser(bytes.NewBuffer(tc.blob)), identity, tc.resource.Version)
				g.Expect(err).NotTo(HaveOccurred())
				blob, err := c.FetchDataByIdentity(context.Background(), identity, tc.resource.Version)
				g.Expect(err).NotTo(HaveOccurred())
				content, err := io.ReadAll(blob)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(content).To(Equal(tc.expected))
			} else {
				exists, err := c.IsCached(context.Background(), identity, tc.resource.Version)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(exists).To(BeFalse())
			}
		})
	}
}
