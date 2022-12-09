// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package oci

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	_ "github.com/distribution/distribution/v3/registry/storage/driver/inmemory"
	"github.com/google/go-containerregistry/pkg/v1/types"
	. "github.com/onsi/gomega"
	ocmapi "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/ocm.software/v3alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	types2 "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/pkg/ocm/fakes"
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

func TestOCIClient_FetchAndCacheResource(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	assert.NoError(t, err)
	fakeClient := fake.NewClientBuilder()

	addr := strings.TrimPrefix(testServer.URL, "http://")
	testCases := []struct {
		name         string
		blob         []byte
		expected     []byte
		snapshotName string
		resource     *ocmapi.Resource
		objects      []client.Object
	}{
		{
			name:     "image",
			blob:     []byte("image"),
			expected: []byte("image"),
			resource: &ocmapi.Resource{
				ElementMeta: ocmapi.ElementMeta{
					Name:    "test-resource-1",
					Version: "v0.0.1",
				},
			},
		},
		{
			name:     "empty image",
			blob:     []byte(""),
			expected: []byte(""),
			resource: &ocmapi.Resource{
				ElementMeta: ocmapi.ElementMeta{
					Name:    "test-resource-2",
					Version: "v0.0.1",
				},
			},
		},
		{
			name:         "snapshot exists",
			blob:         []byte("image"),
			expected:     []byte("image"),
			snapshotName: "test-snapshot",
			resource: &ocmapi.Resource{
				ElementMeta: ocmapi.ElementMeta{
					Name:    "test-resource-3",
					Version: "v0.0.1",
				},
			},
			objects: []client.Object{
				&v1alpha1.Snapshot{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
						Name:      "test-snapshot",
					},
					Spec: v1alpha1.SnapshotSpec{
						Ref: "ref",
					},
					Status: v1alpha1.SnapshotStatus{
						RepositoryURL: fmt.Sprintf(
							"%s/%s/%s",
							addr,
							"test-name",
							"test-resource-3",
						),
						Digest: "sha256:3f45b176c3fa4efa56f1aac522afd99737915e813442dcba1618d99d1a5c4cd8",
						Tag:    "v0.0.1",
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := fakeClient.WithObjects(tc.objects...).WithScheme(scheme).Build()
			fakeOcm := &fakes.MockFetcher{
				FetchedResource: io.NopCloser(bytes.NewBuffer(tc.blob)),
			}
			g := NewWithT(t)
			c := NewClient(fakeOcm, client, addr, scheme)
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
			// Create the Object that the cache can retrieve.
			if len(tc.objects) > 0 {
				err := c.PushResource(context.Background(), ResourceOptions{
					ComponentVersion: obj,
					Resource:         tc.resource,
					SnapshotName:     tc.snapshotName,
					Owner:            obj,
				})
				g.Expect(err).NotTo(HaveOccurred())
			}
			blob, err := c.FetchAndCacheResource(context.Background(), ResourceOptions{
				ComponentVersion: obj,
				Resource:         tc.resource,
				SnapshotName:     tc.snapshotName,
				Owner:            obj,
			})
			g.Expect(err).NotTo(HaveOccurred())
			content, err := io.ReadAll(blob)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(content).To(Equal(tc.expected))

		})
	}
}

func TestOCIClient_PushResource(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	assert.NoError(t, err)
	fakeClient := fake.NewClientBuilder()
	client := fakeClient.WithScheme(scheme).Build()

	addr := strings.TrimPrefix(testServer.URL, "http://")
	testCases := []struct {
		name         string
		blob         []byte
		expected     []byte
		snapshotName string
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
		{
			name:         "should use snapshot name",
			blob:         []byte("with-snapshot"),
			expected:     []byte("with-snapshot"),
			snapshotName: "test-snapshot",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fakeOcm := &fakes.MockFetcher{
				FetchedResource: io.NopCloser(bytes.NewBuffer(tc.blob)),
			}
			g := NewWithT(t)
			c := NewClient(fakeOcm, client, addr, scheme)
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
			err := c.PushResource(context.Background(), ResourceOptions{
				ComponentVersion: obj,
				Resource: &ocmapi.Resource{
					ElementMeta: ocmapi.ElementMeta{
						Name:    "test-resource",
						Version: "v0.0.1",
					},
				},
				SnapshotName: tc.snapshotName,
				Owner:        obj,
			})
			g.Expect(err).NotTo(HaveOccurred())

			snapshot := &v1alpha1.Snapshot{}
			snapshotName := tc.snapshotName
			if snapshotName == "" {
				snapshotName = "test-name-v0-0-1-test-resource-v0-0-1"
			}
			err = client.Get(context.Background(), types2.NamespacedName{Name: snapshotName, Namespace: "default"}, snapshot)
			g.Expect(err).NotTo(HaveOccurred())
			repositoryName := fmt.Sprintf(
				"%s/%s/%s",
				c.ociAddress,
				"test-name",
				"test-resource",
			)
			repo, err := NewRepository(repositoryName, WithInsecure())
			g.Expect(err).NotTo(HaveOccurred())
			layer, err := repo.FetchBlob(snapshot.Status.Digest)
			g.Expect(err).NotTo(HaveOccurred())
			b, err := io.ReadAll(layer)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(b).To(Equal(tc.expected))
		})
	}
}
