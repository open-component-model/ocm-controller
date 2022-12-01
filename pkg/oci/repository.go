package oci

import (
	"fmt"
	"io"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/stream"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

// Repository is a wrapper around go-containerregistry's name.Repository.
// It provides a few convenience methods for fetching and pushing blobs.
type Repository struct {
	name.Repository
	// remoteOpts are the options to use when fetching and pushing blobs.
	remoteOpts []remote.Option
}

// NewRepository returns a new Repository. It points to the given remote repository.
func NewRepository(repositoryName string, opts ...remote.Option) (*Repository, error) {
	repo, err := name.NewRepository(repositoryName)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Repository name %q: %w", repositoryName, err)
	}
	return &Repository{repo, opts}, nil
}

// FetchBlobFrom fetches a blob from the given remote. It accepts a string
// and a boolean to indicate whether the blob should be verified. In order to verify the blob,
// the compressed blob is read and the digest is verified against the digest in the string.
// The uncompressed blob is returned.
// The fetched blob is cached in the repository.
func (r *Repository) FetchBlobFrom(s string, verify bool) (io.ReadCloser, error) {
	l, err := r.fetchBlobFrom(s, verify)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch layer: %w", err)
	}

	// get uncompressed layer
	return l.Uncompressed()
}

func (r *Repository) fetchBlobFrom(s string, verify bool) (v1.Layer, error) {
	ref, err := name.NewDigest(s)
	if err != nil {
		return nil, fmt.Errorf("%s: blob s must be of the form <name@digest>", ref)
	}
	layer, err := remote.Layer(ref, r.remoteOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch layer: %w", err)
	}
	if verify {
		verified, err := verifyBlob(ref, layer)
		if err != nil {
			return nil, fmt.Errorf("failed to verify layer: %w", err)
		}
		if !verified {
			return nil, fmt.Errorf("failed to verify layer")
		}
	}
	// cache the fecched blob in the repository
	err = r.pushBlob(layer)
	if err != nil {
		return layer, fmt.Errorf("failed to cache layer: %w", err)
	}
	return layer, nil
}

// fetchBlob fetches a blob from the repository.
func (r *Repository) fetchBlob(digest string) (v1.Layer, error) {
	ref, err := name.NewDigest(fmt.Sprintf("%s@%s", r.Repository, digest))
	if err != nil {
		return nil, fmt.Errorf("failed to parse digest %q: %w", digest, err)
	}
	return remote.Layer(ref, r.remoteOpts...)
}

// FetchBlob fetches a blob from the given remote.
func (r *Repository) FetchBlob(digest string) (io.ReadCloser, error) {
	l, err := r.fetchBlob(digest)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch layer: %w", err)
	}

	// get uncompressed layer
	return l.Uncompressed()
}

// PushBlob pushes a blob to the repository. It accepts an io.ReadCloser interface.
func (r *Repository) PushBlob(blob io.ReadCloser) error {
	layer, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		return blob, nil
	})
	if err != nil {
		return fmt.Errorf("failed to create layer: %w", err)
	}
	return r.pushBlob(layer)
}

// PushStreamBlob pushes a stream to the repository. It accepts a v1.Image interface.
func (r *Repository) PushStreamBlob(blob io.ReadCloser, digest string) error {
	layer := stream.NewLayer(blob)
	return r.pushBlob(layer)
}

// PushBlob pushes a blob to the repository. It accepts a v1.Layer interface.
func (r *Repository) pushBlob(layer v1.Layer) error {
	return remote.WriteLayer(r.Repository, layer, r.remoteOpts...)
}

// FetchManifest fetches a manifest from the given remote.
func (r *Repository) FetchManifest(s string) ([]byte, error) {
	m, err := r.fetchManifestDescriptor(s)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch manifest: %w", err)
	}
	raw, err := m.RawManifest()
	if err != nil {
		return nil, fmt.Errorf("failed to get raw manifest: %w", err)
	}
	return raw, nil
}

func (r *Repository) fetchManifestDescriptor(s string) (*remote.Descriptor, error) {
	// a manifest reference can be a tag or a digest
	ref, err := name.ParseReference(s)
	if err != nil {
		return nil, fmt.Errorf("failed to parse reference: %w", err)
	}
	// fetch manifest
	// Get does a digest verification
	return remote.Get(ref, r.remoteOpts...)
}

// FetchImage fetches an OCI image from the repository. to the location specified by the
// repository. It accepts a string.
func (r *Repository) FetchImage(reference string) (v1.Image, error) {
	ref, err := parseReference(reference, r)
	if err != nil {
		return nil, fmt.Errorf("failed to parse reference: %w", err)
	}
	m, err := r.fetchManifestDescriptor(ref.String())
	if err != nil {
		return nil, fmt.Errorf("failed to fetch manifest: %w", err)
	}
	// fetch image
	img, err := m.Image()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch image: %w", err)
	}
	return img, nil
}

// FetchImageFrom fetches an OCI image from the given remote.
// It caches the fetched image in the repository.
func (r *Repository) FetchImageFrom(s string) (v1.Image, error) {
	// compute a reference to the image using the repository name and the manifest digest
	var (
		saveRef name.Reference
		err     error
	)
	if strings.Contains(s, ":") {
		refs := strings.Split(s, ":")
		if len(refs) != 2 {
			return nil, fmt.Errorf("failed to parse reference: %s", s)
		}
		saveRef, err = name.NewTag(fmt.Sprintf("%s:%s", r.Repository, refs[1]))
	} else {
		refs := strings.Split(s, "@")
		if len(refs) != 2 {
			return nil, fmt.Errorf("failed to parse reference: %s", s)
		}
		saveRef, err = name.NewDigest(fmt.Sprintf("%s@%s", r.Repository, refs[1]))
	}
	if err != nil {
		return nil, fmt.Errorf("failed to parse reference: %w", err)
	}
	m, err := r.fetchManifestDescriptor(s)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch manifest: %w", err)
	}
	// fetch image
	img, err := m.Image()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch image: %w", err)
	}
	// push image
	if err := r.pushImage(img, saveRef); err != nil {
		return img, fmt.Errorf("failed to push image: %w", err)
	}
	return img, nil
}

// PushImage pushes an OCI image to the repository. It accepts a v1.Image interface.
func (r *Repository) PushImage(image v1.Image, reference string) error {
	ref, err := parseReference(reference, r)
	if err != nil {
		return fmt.Errorf("failed to parse reference: %w", err)
	}
	return r.pushImage(image, ref)
}

// pushImage pushes an OCI image to the repository. It accepts a v1.Image interface.
func (r *Repository) pushImage(image v1.Image, reference name.Reference) error {
	return remote.Write(reference, image, r.remoteOpts...)
}

// IsManifest determines if the given descriptor from a remote points to a manifest,
// i.e. an image or index manifest.
func IsManifest(s string, opts ...remote.Option) (bool, error) {
	ref, err := name.ParseReference(s)
	if err != nil {
		return false, fmt.Errorf("%s: s must be of the form <name@digest> or <name:tag>", ref)
	}
	desc, err := remote.Head(ref, opts...)
	if err != nil {
		return false, fmt.Errorf("failed to get descriptor: %w", err)
	}
	return desc.MediaType.IsIndex() || desc.MediaType.IsImage(), nil
}

func verifyBlob(ref name.Digest, layer v1.Layer) (bool, error) {
	w := NewVerifier(ref.DigestStr())
	rd, er := layer.Compressed()
	if er != nil {
		return false, fmt.Errorf("failed to get layer reader: %w", er)
	}
	ok, err := w.Verify(rd)
	if err != nil {
		return false, fmt.Errorf("failed to verify layer: %w", err)
	}
	if !ok {
		return false, fmt.Errorf("failed to verify layer: %w", err)
	}
	return true, nil
}

func parseReference(reference string, r *Repository) (name.Reference, error) {
	if reference == "" {
		return nil, fmt.Errorf("reference must be specified")
	}
	if strings.Contains(reference, "sha256:") {
		reference = fmt.Sprintf("%s@%s", r.Repository, reference)
	} else {
		reference = fmt.Sprintf("%s:%s", r.Repository, reference)
	}
	ref, err := name.ParseReference(reference)
	if err != nil {
		return nil, fmt.Errorf("failed to parse reference: %w", err)
	}
	return ref, nil
}
