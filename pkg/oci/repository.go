package oci

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/stream"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
)

// Options is a functional option for Repository.
type Option func(o *options) error

type options struct {
	// remoteOpts are the options to use when fetching and pushing blobs.
	remoteOpts []remote.Option
	insecure   bool
}

// WithAuthFromSecret returns an option that configures the repository to use the provided keychain.
func WithAuthFromSecret(ctx context.Context, secret corev1.Secret) Option {
	return func(o *options) error {
		if secret.Type != corev1.SecretTypeDockerConfigJson {
			return fmt.Errorf("secret type %s is not supported", secret.Type)
		}
		chain, err := k8schain.NewFromPullSecrets(ctx, []corev1.Secret{secret})
		if err != nil {
			return fmt.Errorf("failed to create keychain: %w", err)
		}
		o.remoteOpts = append(o.remoteOpts, remote.WithAuthFromKeychain(chain))
		return nil
	}
}

// WithBasicAuth returns an option that configures the repository to use the provided username and password.
func WithBasicAuth(username, password string) Option {
	return func(o *options) error {
		o.remoteOpts = append(o.remoteOpts, remote.WithAuth(&authn.Basic{
			Username: username,
			Password: password,
		}))
		return nil
	}
}

// WithTransportreturns an option that configures the repository to use the provided http.RoundTripper.
func WithTransport(transport http.RoundTripper) Option {
	return func(o *options) error {
		o.remoteOpts = append(o.remoteOpts, remote.WithTransport(transport))
		return nil
	}
}

// WithContext returns an option that configures the repository to use the provided context
func WithContext(ctx context.Context) Option {
	return func(o *options) error {
		o.remoteOpts = append(o.remoteOpts, remote.WithContext(ctx))
		return nil
	}
}

// WithInsecure sets up the registry to use HTTP with --insecure.
func WithInsecure() Option {
	return func(o *options) error {
		o.insecure = true
		return nil
	}
}

// Repository is a wrapper around go-containerregistry's name.Repository.
// It provides a few convenience methods for interacting with OCI registries.
type Repository struct {
	name.Repository
	options
}

// NewRepository returns a new Repository. It points to the given remote repository.
// It accepts a list of options to configure the repository and the underlying remote client.
func NewRepository(repositoryName string, opts ...Option) (*Repository, error) {
	opt, err := makeOptions(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to make options: %w", err)
	}
	repoOpts := make([]name.Option, 0)
	if opt.insecure {
		repoOpts = append(repoOpts, name.Insecure)
	}
	repo, err := name.NewRepository(repositoryName, repoOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Repository name %q: %w", repositoryName, err)
	}
	return &Repository{repo, opt}, nil
}

// FetchBlobFrom fetches a blob from the given remote. It accepts a string
// and a boolean to indicate whether the blob should be verified. In order to verify the blob,
// the compressed blob is read and the digest is verified against the digest in the string.
// The uncompressed blob is returned as a io.ReadCloser.
// The fetched blob is cached if specified in the repository. Even if caching fails, the blob is still returned.
func (r *Repository) FetchBlobFrom(s string, verify, cache bool, opts ...Option) (io.ReadCloser, error) {
	opt, err := makeOptions(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to make options: %w", err)
	}
	l, errf := r.fetchBlobFrom(s, verify, cache, opt.remoteOpts...)
	if l != nil {
		uncompressed, err := l.Uncompressed()
		if err != nil {
			return nil, kerrors.NewAggregate([]error{errf, err})
		}
		return uncompressed, errf
	}
	return nil, errf
}

func (r *Repository) fetchBlobFrom(s string, verify, cache bool, opts ...remote.Option) (v1.Layer, error) {
	ref, err := name.NewDigest(s)
	if err != nil {
		return nil, fmt.Errorf("%s: blob s must be of the form <name@digest>", ref)
	}
	layer, err := remote.Layer(ref, opts...)
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
	if cache {
		// cache the fetched blob in the repository
		err = r.pushBlob(layer)
		if err != nil {
			return layer, fmt.Errorf("failed to cache layer: %w", err)
		}
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

// FetchBlob fetches a blob from the repository.
func (r *Repository) FetchBlob(digest string) (io.ReadCloser, error) {
	l, err := r.fetchBlob(digest)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch layer: %w", err)
	}
	// get uncompressed layer
	return l.Uncompressed()
}

// PushBlob pushes a blob to the repository. It accepts a slice of bytes.
// A media type can be specified to override the default media type.
// Default media type is "application/vnd.oci.image.layer.v1.tar+gzip".
func (r *Repository) PushBlob(blob []byte, mediaType string) (*ocispec.Descriptor, error) {
	layer, err := computeBlob(blob, mediaType)
	if err != nil {
		return nil, fmt.Errorf("failed to create layer: %w", err)
	}
	err = r.pushBlob(layer)
	if err != nil {
		return nil, fmt.Errorf("failed to push layer: %w", err)
	}
	desc, err := layerToOCIDescriptor(layer)
	if err != nil {
		return nil, fmt.Errorf("failed to get layer descriptor: %w", err)
	}
	return desc, nil
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
	}
	if len(annotations) == 0 {
		return nil, nil, fmt.Errorf("no matching annotations found")
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

// FetchManifestFrom fetches a manifest from the given remote.
// It returns the manifest as an oci.Manifest struct and the raw manifest as a byte slice.
// The oci.Manifest struct can be used to retrieve the layers digests.
func FetchManifestFrom(s string, opts ...Option) (*ocispec.Manifest, []byte, error) {
	opt, err := makeOptions(opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to make options: %w", err)
	}
	m, err := fetchManifestDescriptorFrom(s, opt.remoteOpts...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch manifest: %w", err)
	}
	raw, err := m.RawManifest()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get raw manifest: %w", err)
	}
	desc, err := manifestToOCIDescriptor(m)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get manifest descriptor: %w", err)
	}
	return desc, raw, nil
}

func fetchManifestDescriptorFrom(s string, opts ...remote.Option) (*remote.Descriptor, error) {
	// a manifest reference can be a tag or a digest
	ref, err := name.ParseReference(s)
	if err != nil {
		return nil, fmt.Errorf("failed to parse reference: %w", err)
	}
	// fetch manifest
	// Get performs a digest verification
	return remote.Get(ref, opts...)
}

// PushImage pushes a blob to the repository as an OCI image.
// It accepts a media type and a byte slice as the blob.
// Default media type is "application/vnd.oci.image.layer.v1.tar+gzip".
// Annotations can be passed to the image manifest.
func (r *Repository) PushImage(reference string, blob []byte, mediaType string, annotations map[string]string) error {
	ref, err := parseReference(reference, r)
	if err != nil {
		return fmt.Errorf("failed to parse reference: %w", err)
	}
	image, err := computeImage(blob, mediaType)
	if err != nil {
		return fmt.Errorf("failed to compute image: %w", err)
	}
	var annotatedImage v1.Image
	if len(annotations) > 0 {
		annotatedImage = mutate.Annotations(image, annotations).(v1.Image)
	}
	if annotatedImage != nil {
		return r.pushImage(annotatedImage, ref)
	}
	return r.pushImage(image, ref)
}

// pushImage pushes an OCI image to the repository. It accepts a v1.Image interface.
func (r *Repository) pushImage(image v1.Image, reference name.Reference) error {
	return remote.Write(reference, image, r.remoteOpts...)
}

// IsManifest determines if the given descriptor from a remote points to a manifest,
// i.e. an image or index manifest.
func IsManifest(s string, opts ...Option) (bool, error) {
	opt, err := makeOptions(opts...)
	if err != nil {
		return false, fmt.Errorf("failed to make options: %w", err)
	}
	ref, err := name.ParseReference(s)
	if err != nil {
		return false, fmt.Errorf("%s: s must be of the form <name@digest> or <name:tag>", ref)
	}
	// Head merely checks if the reference exists
	desc, err := remote.Head(ref, opt.remoteOpts...)
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

func computeImage(data []byte, mediaType string) (v1.Image, error) {
	l, err := computeBlob(data, mediaType)
	if err != nil {
		return nil, err
	}
	return mutate.AppendLayers(empty.Image, l)
}

func computeBlob(data []byte, mediaType string) (v1.Layer, error) {
	t := types.MediaType(mediaType)
	if t == "" {
		t = types.OCILayer
	}
	l, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(data)), nil
	}, tarball.WithMediaType(t))

	if err != nil {
		return nil, err
	}
	return l, nil
}

func getDescriptor(manifest []byte) (*v1.Descriptor, error) {
	desc := &v1.Descriptor{}
	if err := json.Unmarshal(manifest, desc); err != nil {
		return nil, fmt.Errorf("failed to unmarshal manifest: %w", err)
	}
	return desc, nil
}
