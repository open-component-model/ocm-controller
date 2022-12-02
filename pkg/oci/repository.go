package oci

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/stream"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
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

// Repository is a wrapper around go-containerregistry's name.Repository.
// It provides a few convenience methods for interacting with OCI registries.
type Repository struct {
	name.Repository
	options
}

// NewRepository returns a new Repository. It points to the given remote repository.
// It accepts a list of options to configure the repository and the underlying remote client.
func NewRepository(repositoryName string, opts ...Option) (*Repository, error) {
	repo, err := name.NewRepository(repositoryName)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Repository name %q: %w", repositoryName, err)
	}
	opt, err := makeOptions(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to make options: %w", err)
	}
	return &Repository{repo, opt}, nil
}

// FetchBlobFrom fetches a blob from the given remote. It accepts a string
// and a boolean to indicate whether the blob should be verified. In order to verify the blob,
// the compressed blob is read and the digest is verified against the digest in the string.
// The uncompressed blob is returned as a io.ReadCloser.
// The fetched blob is cached in the repository. Even if caching fails, the blob is still returned.
func (r *Repository) FetchBlobFrom(s string, verify bool, opts ...Option) (io.ReadCloser, error) {
	opt, err := makeOptions(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to make options: %w", err)
	}
	l, errf := r.fetchBlobFrom(s, verify, opt.remoteOpts...)
	if l != nil {
		uncompressed, err := l.Uncompressed()
		if err != nil {
			return nil, kerrors.NewAggregate([]error{errf, err})
		}
		return uncompressed, errf
	}
	return nil, errf
}

func (r *Repository) fetchBlobFrom(s string, verify bool, opts ...remote.Option) (v1.Layer, error) {
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

// FetchBlob fetches a blob from the repository.
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

// PushStreamBlob pushes by streaming a blob to the repository. It accepts an io.ReadCloser interface.
func (r *Repository) PushStreamBlob(blob io.ReadCloser, digest string) error {
	layer := stream.NewLayer(blob)
	return r.pushBlob(layer)
}

// pushBlob pushes a blob to the repository. It accepts a v1.Layer interface.
func (r *Repository) pushBlob(layer v1.Layer) error {
	return remote.WriteLayer(r.Repository, layer, r.remoteOpts...)
}

// FetchManifest fetches a manifest from the repository.
// It returns the manifest as an oci.Manifest struct and the raw manifest as a byte slice.
// The oci.Manifest struct can be used to retrieve the layers digests.
func (r *Repository) FetchManifest(reference string) (*ocispec.Manifest, []byte, error) {
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
	desc, err := manifestToOCIDescriptor(m)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get manifest descriptor: %w", err)
	}
	return desc, raw, nil
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

// FetchImage fetches an OCI image from the repository. It returns the image as a v1.Image interface.
// It accepts a string to indicate the reference to the image.
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
	return m.Image()
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
