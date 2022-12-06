// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package oci

import (
	"io"
	"strings"
	"testing"
	"time"

	_ "github.com/distribution/distribution/v3/registry/storage/driver/inmemory"
	"github.com/google/go-containerregistry/pkg/v1/types"
	. "github.com/onsi/gomega"
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
			layer, err := computeBlob(tc.blob, string(types.OCILayer))
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

func TestRepository_Image(t *testing.T) {
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
			err = repo.PushImage("latest", tc.blob, string(types.OCILayer), map[string]string{
				"org.opencontainers.artifact.created": time.Now().UTC().Format(time.RFC3339),
			})
			g.Expect(err).NotTo(HaveOccurred())

			// fetch the image layer from the registry
			fetchedManifest, _, err := repo.FetchManifest("latest", []string{string("org.opencontainers.artifact.created")})
			g.Expect(err).NotTo(HaveOccurred())
			layers := fetchedManifest.Layers
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(len(layers)).To(Equal(1))
			digest := layers[0].Digest
			layer, err := repo.FetchBlob(digest.String())
			g.Expect(err).NotTo(HaveOccurred())
			b, err := io.ReadAll(layer)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(b).To(Equal(tc.expected))
		})
	}
}

func TestRepository_FetchBlobFromRemote(t *testing.T) {
	addr := strings.TrimPrefix(testServer.URL, "http://")
	g := NewWithT(t)
	// we will fetch the latest busybox image from dockerhub
	remoteImage := "docker.io/library/busybox:latest"
	// and cache it in our local registry
	repoName := "library/busybox"
	repo, err := NewRepository(addr + "/" + repoName)
	g.Expect(err).NotTo(HaveOccurred())
	// fetch the image from the remote registry
	desc, _, err := FetchManifestFrom(remoteImage)
	g.Expect(err).NotTo(HaveOccurred())
	layers := desc.Layers
	g.Expect(err).NotTo(HaveOccurred())
	digest := layers[0].Digest
	g.Expect(err).NotTo(HaveOccurred())
	// fetch the blob from the remote registry
	rc, err := repo.FetchBlobFrom("docker.io/library/busybox@"+digest.String(), true, true)
	g.Expect(err).NotTo(HaveOccurred())
	b, err := io.ReadAll(rc)
	g.Expect(err).NotTo(HaveOccurred())
	// fetch the blob from the local registry
	cachedRc, err := repo.FetchBlob(digest.String())
	g.Expect(err).NotTo(HaveOccurred())
	cachedB, err := io.ReadAll(cachedRc)
	g.Expect(err).NotTo(HaveOccurred())
	// the blobs should match
	g.Expect(b).To(Equal(cachedB))
}

func TestRepository_IsManifest(t *testing.T) {
	addr := strings.TrimPrefix(testServer.URL, "http://")
	testCases := []struct {
		name     string
		blob     []byte
		manifest bool
	}{
		{
			name:     "image",
			blob:     []byte("image"),
			manifest: true,
		},
		{
			name:     "empty image",
			blob:     []byte(""),
			manifest: true,
		},
		{
			name:     "blob",
			blob:     []byte("blob"),
			manifest: false,
		},
		{
			name:     "empty blob",
			blob:     []byte(""),
			manifest: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			// create a repository client
			repoName := generateRandomName("test-is-manifest")
			repo, err := NewRepository(addr + "/" + repoName)
			g.Expect(err).NotTo(HaveOccurred())

			if tc.manifest {
				// push the image to the registry
				err = repo.PushImage("latest", tc.blob, string(types.OCILayer), map[string]string{
					"org.opencontainers.artifact.created": time.Now().UTC().Format(time.RFC3339),
				})
				g.Expect(err).NotTo(HaveOccurred())
				// check if we have a manifest
				isManifest, err := IsManifest(addr + "/" + repoName + ":latest")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(isManifest).To(Equal(tc.manifest))
				return
			}

			//compute blob
			layer, err := computeBlob(tc.blob, string(types.OCILayer))
			g.Expect(err).NotTo(HaveOccurred())
			// push the blob to the registry
			err = repo.pushBlob(layer)
			g.Expect(err).NotTo(HaveOccurred())
			// check if we have a manifest
			digest, err := layer.Digest()
			g.Expect(err).NotTo(HaveOccurred())
			isManifest, err := IsManifest(addr + "/" + repoName + "@" + digest.String())
			g.Expect(err).To(HaveOccurred())
			g.Expect(isManifest).To(Equal(tc.manifest))
		})
	}
}
