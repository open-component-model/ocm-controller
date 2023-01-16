// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package oci

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	ociname "github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/stream"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
)

// Option is a functional option for Repository.
type Option func(o *options) error

type options struct {
	// remoteOpts are the options to use when fetching and pushing blobs.
	remoteOpts []remote.Option
	insecure   bool
}

// WithInsecure sets up the registry to use HTTP with --insecure.
func WithInsecure() Option {
	return func(o *options) error {
		o.insecure = true
		return nil
	}
}

// ResourceOptions contains all parameters necessary to fetch / push resources.
type ResourceOptions struct {
	ComponentVersion *v1alpha1.ComponentVersion
	Resource         v1alpha1.ResourceRef
	Owner            metav1.Object
	SnapshotName     string
}

// Client implements the caching layer and the OCI layer.
type Client struct {
	OCIRepositoryAddr string
}

// NewClient creates a new OCI Client.
func NewClient(ociAddress string) *Client {
	return &Client{
		OCIRepositoryAddr: ociAddress,
	}
}

// Repository is a wrapper around go-container registry's name.Repository.
// It provides a few convenience methods for interacting with OCI registries.
type Repository struct {
	ociname.Repository
	options
}

// NewRepository returns a new Repository. It points to the given remote repository.
// It accepts a list of options to configure the repository and the underlying remote client.
func NewRepository(repositoryName string, opts ...Option) (*Repository, error) {
	opt, err := makeOptions(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to make options: %w", err)
	}
	repoOpts := make([]ociname.Option, 0)
	if opt.insecure {
		repoOpts = append(repoOpts, ociname.Insecure)
	}
	repo, err := ociname.NewRepository(repositoryName, repoOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Repository name %q: %w", repositoryName, err)
	}
	return &Repository{repo, opt}, nil
}

// PushData takes a blob of data and caches it using OCI as a background.
func (c *Client) PushData(ctx context.Context, data io.ReadCloser, name, tag string) (string, error) {
	repositoryName := fmt.Sprintf("%s/%s", c.OCIRepositoryAddr, name)
	repo, err := NewRepository(repositoryName, WithInsecure())
	if err != nil {
		return "", fmt.Errorf("failed create new repository: %w", err)
	}

	manifest, err := repo.PushStreamingImage(tag, data, "", nil)
	if err != nil {
		return "", fmt.Errorf("failed to push image: %w", err)
	}

	layers := manifest.Layers
	if len(layers) == 0 {
		return "", fmt.Errorf("no layers returned by manifest: %w", err)
	}

	return layers[0].Digest.String(), nil
}

// FetchDataByIdentity fetches an existing resource. Errors if there is no resource available. It's advised to call IsCached
// before fetching. Returns the digest of the resource alongside the data for further processing.
func (c *Client) FetchDataByIdentity(ctx context.Context, name, tag string) (io.ReadCloser, string, error) {
	repositoryName := fmt.Sprintf("%s/%s", c.OCIRepositoryAddr, name)
	repo, err := NewRepository(repositoryName, WithInsecure())
	if err != nil {
		return nil, "", fmt.Errorf("failed to get repository: %w", err)
	}

	manifest, _, err := repo.FetchManifest(tag, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch manifest to obtain layers: %w", err)
	}
	layers := manifest.Layers
	if len(layers) == 0 {
		return nil, "", fmt.Errorf("layers for repository is empty")
	}

	digest := layers[0].Digest

	reader, err := repo.FetchBlob(digest.String())
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch reader for digest of the 0th layer: %w", err)
	}

	return reader, digest.String(), nil
}

func (c *Client) FetchDataByDigest(ctx context.Context, name, digest string) (io.ReadCloser, error) {
	repositoryName := fmt.Sprintf("%s/%s", c.OCIRepositoryAddr, name)

	repo, err := NewRepository(repositoryName, WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}

	return repo.FetchBlob(digest)
}

func (c *Client) IsCached(ctx context.Context, name, tag string) (bool, error) {
	repositoryName := fmt.Sprintf("%s/%s", c.OCIRepositoryAddr, name)
	reference, err := ociname.ParseReference(fmt.Sprintf("%s/%s", repositoryName, tag))
	if err != nil {
		return false, fmt.Errorf("failed to parse repository and tag name: %w", err)
	}

	if _, err := remote.Head(reference); err != nil {
		return false, nil
	}
	return true, nil
}

// fetchBlob fetches a blob from the repository.
func (r *Repository) fetchBlob(digest string) (v1.Layer, error) {
	ref, err := ociname.NewDigest(fmt.Sprintf("%s@%s", r.Repository, digest))
	if err != nil {
		return nil, fmt.Errorf("failed to parse digest %q: %w", digest, err)
	}
	return remote.Layer(ref, r.remoteOpts...)
}

// FetchBlob fetches a blob from the repository.
func (r *Repository) FetchBlob(digest string) (io.ReadCloser, error) {
	l, err := r.fetchBlob(digest)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch layer: %w", err)
	}
	return l.Uncompressed()
}

// PushStreamBlob pushes by streaming a blob to the repository. It accepts an io.ReadCloser interface.
// A media type can be specified to override the default media type.
// Default media type is "application/vnd.oci.image.layer.v1.tar+gzip".
func (r *Repository) PushStreamBlob(blob io.ReadCloser, mediaType string) (*ocispec.Descriptor, error) {
	t := types.MediaType(mediaType)
	if t == "" {
		t = types.OCILayer
	}
	layer := stream.NewLayer(blob, stream.WithMediaType(t))
	err := r.pushBlob(layer)
	if err != nil {
		return nil, fmt.Errorf("failed to push layer: %w", err)
	}
	desc, err := layerToOCIDescriptor(layer)
	if err != nil {
		return nil, fmt.Errorf("failed to get layer descriptor: %w", err)
	}
	return desc, nil
}

// pushBlob pushes a blob to the repository. It accepts a v1.Layer interface.
func (r *Repository) pushBlob(layer v1.Layer) error {
	return remote.WriteLayer(r.Repository, layer, r.remoteOpts...)
}

// PushStreamingImage pushes a reader to the repository as a streaming OCI image.
// It accepts a media type and a byte slice as the blob.
// Default media type is "application/vnd.oci.image.layer.v1.tar+gzip".
// Annotations can be passed to the image manifest.
func (r *Repository) PushStreamingImage(reference string, reader io.ReadCloser, mediaType string, annotations map[string]string) (*v1.Manifest, error) {
	ref, err := parseReference(reference, r)
	if err != nil {
		return nil, fmt.Errorf("failed to parse reference: %w", err)
	}
	image, err := computeStreamImage(reader, mediaType)
	if err != nil {
		return nil, fmt.Errorf("failed to compute image: %w", err)
	}
	if len(annotations) > 0 {
		image = mutate.Annotations(image, annotations).(v1.Image)
	}
	if err := r.pushImage(image, ref); err != nil {
		return nil, fmt.Errorf("failed to push image: %w", err)
	}
	return image.Manifest()
}

// pushImage pushes an OCI image to the repository. It accepts a v1.RepositoryURL interface.
func (r *Repository) pushImage(image v1.Image, reference ociname.Reference) error {
	return remote.Write(reference, image, r.remoteOpts...)
}

// FetchManifest fetches a manifest from the repository.
// It returns the manifest as an oci.Manifest struct and the raw manifest as a byte slice.
// The oci.Manifest struct can be used to retrieve the layers digests.
// Optionally, the manifest annotations can be verified against the given slice of strings keys.
func (r *Repository) FetchManifest(reference string, filters []string) (*ocispec.Manifest, []byte, error) {
	ref, err := parseReference(reference, r)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse reference: %w", err)
	}
	m, err := r.fetchManifestDescriptor(ref.String())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch manifest: %w", err)
	}
	raw, err := m.RawManifest()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get raw manifest: %w", err)
	}
	//check if the manifest annotations match the given filters
	var annotations map[string]string
	if len(filters) > 0 {
		// get desciptor from manifest
		desc, err := getDescriptor(raw)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get descriptor: %w", err)
		}
		annotations = filterAnnotations(desc.Annotations, filters)
		if len(annotations) == 0 {
			return nil, nil, fmt.Errorf("no matching annotations found")
		}
	}

	desc, err := manifestToOCIDescriptor(m)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get manifest descriptor: %w", err)
	}
	return desc, raw, nil
}

func (r *Repository) fetchManifestDescriptor(s string) (*remote.Descriptor, error) {
	return fetchManifestDescriptorFrom(s, r.remoteOpts...)
}

// manifestToOCIDescriptor converts a manifest to an OCI Manifest struct.
// It contains the layers descriptors.
func manifestToOCIDescriptor(m *remote.Descriptor) (*ocispec.Manifest, error) {
	ociManifest := &ocispec.Manifest{}
	ociManifest.MediaType = string(m.MediaType)
	image, err := m.Image()
	if err != nil {
		return nil, fmt.Errorf("failed to get image: %w", err)
	}
	layers, err := image.Layers()
	if err != nil {
		return nil, fmt.Errorf("failed to get layers: %w", err)
	}
	for _, layer := range layers {
		ociLayer, err := layerToOCIDescriptor(layer)
		if err != nil {
			return nil, fmt.Errorf("failed to get layer: %w", err)
		}
		ociManifest.Layers = append(ociManifest.Layers, *ociLayer)
	}
	return ociManifest, nil
}

func fetchManifestDescriptorFrom(s string, opts ...remote.Option) (*remote.Descriptor, error) {
	// a manifest reference can be a tag or a digest
	ref, err := ociname.ParseReference(s)
	if err != nil {
		return nil, fmt.Errorf("failed to parse reference: %w", err)
	}
	// fetch manifest
	// Get performs a digest verification
	return remote.Get(ref, opts...)
}

func parseReference(reference string, r *Repository) (ociname.Reference, error) {
	if reference == "" {
		return nil, fmt.Errorf("reference must be specified")
	}
	if strings.Contains(reference, "sha256:") {
		reference = fmt.Sprintf("%s@%s", r.Repository, reference)
	} else {
		reference = fmt.Sprintf("%s:%s", r.Repository, reference)
	}
	ref, err := ociname.ParseReference(reference)
	if err != nil {
		return nil, fmt.Errorf("failed to parse reference: %w", err)
	}
	return ref, nil
}

// layerToOCIDescriptor converts a layer to an OCI Layer struct.
func layerToOCIDescriptor(layer v1.Layer) (*ocispec.Descriptor, error) {
	ociLayer := &ocispec.Descriptor{}
	mediaType, err := layer.MediaType()
	if err != nil {
		return nil, fmt.Errorf("failed to get media type: %w", err)
	}
	d, err := layer.Digest()
	if err != nil {
		return nil, fmt.Errorf("failed to get digest: %w", err)
	}
	size, err := layer.Size()
	if err != nil {
		return nil, fmt.Errorf("failed to get size: %w", err)
	}
	ociLayer.MediaType = string(mediaType)
	ociLayer.Digest = digest.NewDigestFromHex(d.Algorithm, d.Hex)
	ociLayer.Size = size
	return ociLayer, nil
}

func makeOptions(opts ...Option) (options, error) {
	opt := options{}
	for _, o := range opts {
		if err := o(&opt); err != nil {
			return options{}, fmt.Errorf("failed to apply option: %w", err)
		}
	}
	return opt, nil
}

// filterAnnotations filters the annotations of a map of annotations.
// It returns a map of annotations that match the given entries.
func filterAnnotations(annotations map[string]string, filters []string) map[string]string {
	filtered := make(map[string]string)
	for k, v := range annotations {
		for _, match := range filters {
			if strings.EqualFold(k, match) {
				filtered[k] = v
			}
		}
	}
	return filtered
}

func computeStreamImage(reader io.ReadCloser, mediaType string) (v1.Image, error) {
	l, err := computeStreamBlob(reader, mediaType)
	if err != nil {
		return nil, err
	}
	return mutate.AppendLayers(empty.Image, l)
}

func computeStreamBlob(reader io.ReadCloser, mediaType string) (v1.Layer, error) {
	t := types.MediaType(mediaType)
	if t == "" {
		t = types.OCILayer
	}
	l := stream.NewLayer(reader, stream.WithMediaType(t))
	return l, nil
}

func getDescriptor(manifest []byte) (*v1.Descriptor, error) {
	desc := &v1.Descriptor{}
	if err := json.Unmarshal(manifest, desc); err != nil {
		return nil, fmt.Errorf("failed to unmarshal manifest: %w", err)
	}
	return desc, nil
}
