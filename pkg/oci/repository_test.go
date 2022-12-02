package oci

import (
	"bytes"
	"io"
	"strings"
	"testing"

	_ "github.com/distribution/distribution/v3/registry/storage/driver/inmemory"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
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
			layer, err := computeBlob([]byte(tc.blob))
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

			// compute a blob
			img, err := computeImage([]byte(tc.blob))
			g.Expect(err).NotTo(HaveOccurred())

			// push image to the registry
			err = repo.PushImage(img, "latest")
			g.Expect(err).NotTo(HaveOccurred())

			// fetch the image from the registry
			fetchedImage, err := repo.FetchImage("latest")
			g.Expect(err).NotTo(HaveOccurred())
			layers, err := fetchedImage.Layers()
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(len(layers)).To(Equal(1))
			rc, err := layers[0].Uncompressed()
			g.Expect(err).NotTo(HaveOccurred())
			b, err := io.ReadAll(rc)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(b).To(Equal(tc.expected))
		})
	}
}

func TestRepository_FetchImageFromRemote(t *testing.T) {
	addr := strings.TrimPrefix(testServer.URL, "http://")
	g := NewWithT(t)
	// we will fetch the latest busybox image from dockerhub
	remoteImage := "docker.io/library/busybox:latest"
	// and cache it in our local registry
	repoName := "library/busybox"
	repo, err := NewRepository(addr + "/" + repoName)
	g.Expect(err).NotTo(HaveOccurred())
	// fetch the image from the remote registry
	img, err := repo.FetchImageFrom(remoteImage)
	g.Expect(err).NotTo(HaveOccurred())
	// fetch the image from the local registry
	cachedImage, err := repo.FetchImage("latest")
	g.Expect(err).NotTo(HaveOccurred())
	d, err := img.Digest()
	g.Expect(err).NotTo(HaveOccurred())
	cachedDigest, err := cachedImage.Digest()
	g.Expect(err).NotTo(HaveOccurred())
	// the digests should match
	g.Expect(d).To(Equal(cachedDigest))
	manifest, err := img.RawManifest()
	g.Expect(err).NotTo(HaveOccurred())
	cachedManifest, err := cachedImage.RawManifest()
	g.Expect(err).NotTo(HaveOccurred())
	// the manifests should match
	g.Expect(manifest).To(Equal(cachedManifest))
	layers, err := img.Layers()
	g.Expect(err).NotTo(HaveOccurred())
	cachedLayers, err := cachedImage.Layers()
	g.Expect(err).NotTo(HaveOccurred())
	// the layers should match
	g.Expect(len(layers)).To(Equal(len(cachedLayers)))
	for i := range layers {
		l, err := layers[i].Digest()
		g.Expect(err).NotTo(HaveOccurred())
		cachedL, err := cachedLayers[i].Digest()
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(l).To(Equal(cachedL))
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
	rc, err := repo.FetchBlobFrom("docker.io/library/busybox@"+digest.String(), true)
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
				img, err := computeImage(tc.blob)
				g.Expect(err).NotTo(HaveOccurred())
				// push the image to the registry
				err = repo.PushImage(img, "latest")
				g.Expect(err).NotTo(HaveOccurred())
				// check if we have a manifest
				isManifest, err := IsManifest(addr + "/" + repoName + ":latest")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(isManifest).To(Equal(tc.manifest))
				return
			}

			//compute blob
			layer, err := computeBlob(tc.blob)
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

func computeImage(data []byte) (v1.Image, error) {
	l, err := computeBlob(data)
	if err != nil {
		return nil, err
	}
	return mutate.AppendLayers(empty.Image, l)
}

func computeBlob(data []byte) (v1.Layer, error) {
	l, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(data)), nil
	})

	if err != nil {
		return nil, err
	}
	return l, nil
}
