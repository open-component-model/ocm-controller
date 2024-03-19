// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package oci

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	ociname "github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/google/go-containerregistry/pkg/v1/stream"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"helm.sh/helm/v3/pkg/registry"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
)

// Option is a functional option for Repository.
type Option func(o *options) error

type options struct {
	// remoteOpts are the options to use when fetching and pushing blobs.
	remoteOpts []remote.Option
}

// ResourceOptions contains all parameters necessary to fetch / push resources.
type ResourceOptions struct {
	ComponentVersion *v1alpha1.ComponentVersion
	Resource         v1alpha1.ResourceReference
	Owner            metav1.Object
	SnapshotName     string
}

// ClientOptsFunc options are used to leave the cache backwards compatible.
// If the certificate isn't defined, we will use `WithInsecure`.
type ClientOptsFunc func(opts *Client)

// WithCertificateSecret defines the name of the secret holding the certificates.
func WithCertificateSecret(name string) ClientOptsFunc {
	return func(opts *Client) {
		opts.CertSecretName = name
	}
}

// WithNamespace sets up certificates for the client.
func WithNamespace(namespace string) ClientOptsFunc {
	return func(opts *Client) {
		opts.Namespace = namespace
	}
}

// WithInsecureSkipVerify sets up certificates for the client.
func WithInsecureSkipVerify(value bool) ClientOptsFunc {
	return func(opts *Client) {
		opts.InsecureSkipVerify = value
	}
}

// WithClient sets up certificates for the client.
func WithClient(client client.Client) ClientOptsFunc {
	return func(opts *Client) {
		opts.Client = client
	}
}

// Client implements the caching layer and the OCI layer.
type Client struct {
	Client             client.Client
	OCIRepositoryAddr  string
	InsecureSkipVerify bool
	Namespace          string
	CertSecretName     string

	certPem []byte
	keyPem  []byte
	ca      []byte
}

// WithTransport sets up insecure TLS so the library is forced to use HTTPS.
func (c *Client) WithTransport(ctx context.Context) Option {
	return func(o *options) error {
		if c.InsecureSkipVerify {
			return nil
		}

		if c.certPem == nil && c.keyPem == nil {
			if err := c.setupCertificates(ctx); err != nil {
				return fmt.Errorf("failed to set up certificates for transport: %w", err)
			}
		}

		o.remoteOpts = append(o.remoteOpts, remote.WithTransport(c.constructTLSRoundTripper()))

		return nil
	}
}

func (c *Client) setupCertificates(ctx context.Context) error {
	if c.Client == nil {
		return fmt.Errorf("client must not be nil if certificate is requested, please set WithClient when creating the oci cache")
	}
	registryCerts := &corev1.Secret{}
	if err := c.Client.Get(ctx, apitypes.NamespacedName{Name: c.CertSecretName, Namespace: c.Namespace}, registryCerts); err != nil {
		return fmt.Errorf("unable to find the secret containing the registry certificates: %w", err)
	}

	certFile, ok := registryCerts.Data["tls.crt"]
	if !ok {
		return fmt.Errorf("tls.crt data not found in registry certificate secret")
	}

	keyFile, ok := registryCerts.Data["tls.key"]
	if !ok {
		return fmt.Errorf("tls.key data not found in registry certificate secret")
	}

	caFile, ok := registryCerts.Data["ca.crt"]
	if !ok {
		return fmt.Errorf("ca.crt data not found in registry certificate secret")
	}

	c.certPem = certFile
	c.keyPem = keyFile
	c.ca = caFile

	return nil
}

func (c *Client) constructTLSRoundTripper() http.RoundTripper {
	tlsConfig := &tls.Config{} //nolint:gosec // must provide lower version for quay.io
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(c.ca)

	tlsConfig.Certificates = []tls.Certificate{
		{
			Certificate: [][]byte{c.certPem},
			PrivateKey:  c.keyPem,
		},
	}

	tlsConfig.RootCAs = caCertPool
	tlsConfig.InsecureSkipVerify = c.InsecureSkipVerify

	// Create a new HTTP transport with the TLS configuration
	return &http.Transport{
		TLSClientConfig: tlsConfig,
	}
}

// NewClient creates a new OCI Client.
func NewClient(ociAddress string, opts ...ClientOptsFunc) *Client {
	c := &Client{
		OCIRepositoryAddr: ociAddress,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
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

	repo, err := ociname.NewRepository(repositoryName, repoOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Repository name %q: %w", repositoryName, err)
	}

	return &Repository{repo, opt}, nil
}

// PushData takes a blob of data and caches it using OCI as a background.
func (c *Client) PushData(ctx context.Context, data io.ReadCloser, mediaType, name, tag string) (string, int64, error) {
	repositoryName := fmt.Sprintf("%s/%s", c.OCIRepositoryAddr, name)
	repo, err := NewRepository(repositoryName, c.WithTransport(ctx))
	if err != nil {
		return "", -1, fmt.Errorf("failed create new repository: %w", err)
	}

	manifest, err := repo.PushStreamingImage(tag, data, mediaType, nil)
	if err != nil {
		return "", -1, fmt.Errorf("failed to push image: %w", err)
	}

	layers := manifest.Layers
	if len(layers) == 0 {
		return "", -1, fmt.Errorf("no layers returned by manifest: %w", err)
	}

	return layers[0].Digest.String(), layers[0].Size, nil
}

// FetchDataByIdentity fetches an existing resource. Errors if there is no resource available. It's advised to call IsCached
// before fetching. Returns the digest of the resource alongside the data for further processing.
func (c *Client) FetchDataByIdentity(ctx context.Context, name, tag string) (io.ReadCloser, string, int64, error) {
	logger := log.FromContext(ctx).WithName("cache")
	repositoryName := fmt.Sprintf("%s/%s", c.OCIRepositoryAddr, name)
	logger.V(v1alpha1.LevelDebug).Info("cache hit for data", "name", name, "tag", tag, "repository", repositoryName)
	repo, err := NewRepository(repositoryName, c.WithTransport(ctx))
	if err != nil {
		return nil, "", -1, fmt.Errorf("failed to get repository: %w", err)
	}

	manifest, _, err := repo.FetchManifest(tag, nil)
	if err != nil {
		return nil, "", -1, fmt.Errorf("failed to fetch manifest to obtain layers: %w", err)
	}
	logger.V(v1alpha1.LevelDebug).Info("got the manifest", "manifest", manifest)
	layers := manifest.Layers
	if len(layers) == 0 {
		return nil, "", -1, fmt.Errorf("layers for repository is empty")
	}

	digest := layers[0].Digest

	reader, err := repo.FetchBlob(digest.String())
	if err != nil {
		return nil, "", -1, fmt.Errorf("failed to fetch reader for digest of the 0th layer: %w", err)
	}

	// decompresses the data coming from the cache. Because a streaming layer doesn't support decompression
	// and a static layer returns the data AS IS, we have to decompress it ourselves.
	return reader, digest.String(), layers[0].Size, nil
}

// FetchDataByDigest returns a reader for a given digest.
func (c *Client) FetchDataByDigest(ctx context.Context, name, digest string) (io.ReadCloser, error) {
	repositoryName := fmt.Sprintf("%s/%s", c.OCIRepositoryAddr, name)

	repo, err := NewRepository(repositoryName, c.WithTransport(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}

	reader, err := repo.FetchBlob(digest)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch blob: %w", err)
	}

	// decompresses the data coming from the cache. Because a streaming layer doesn't support decompression
	// and a static layer returns the data AS IS, we have to decompress it ourselves.
	return reader, nil
}

// IsCached returns whether a certain tag with a given name exists in cache.
func (c *Client) IsCached(ctx context.Context, name, tag string) (bool, error) {
	repositoryName := fmt.Sprintf("%s/%s", c.OCIRepositoryAddr, name)

	repo, err := NewRepository(repositoryName, c.WithTransport(ctx))
	if err != nil {
		return false, fmt.Errorf("failed to get repository: %w", err)
	}

	return repo.head(tag)
}

// DeleteData removes a specific tag from the cache.
func (c *Client) DeleteData(ctx context.Context, name, tag string) error {
	repositoryName := fmt.Sprintf("%s/%s", c.OCIRepositoryAddr, name)
	repo, err := NewRepository(repositoryName, c.WithTransport(ctx))
	if err != nil {
		return fmt.Errorf("failed create new repository: %w", err)
	}

	return repo.deleteTag(tag)
}

// head does an authenticated call with the repo context to see if a tag in a repository already exists or not.
func (r *Repository) head(tag string) (bool, error) {
	reference, err := ociname.ParseReference(fmt.Sprintf("%s:%s", r.Repository, tag))
	if err != nil {
		return false, fmt.Errorf("failed to parse repository and tag name: %w", err)
	}

	if _, err := remote.Head(reference, r.remoteOpts...); err != nil {
		terr := &transport.Error{}
		if ok := errors.As(err, &terr); ok {
			if terr.StatusCode == http.StatusNotFound {
				return false, nil
			}
		}

		return false, err
	}

	return true, nil
}

// deleteTag fetches the latest digest for a tag. This will delete the whole Manifest.
// This is done because docker registry doesn't technically support deleting a single Tag.
// But since we have a 1:1 relationship between a tag and a manifest, it's safe to delete
// the complete manifest.
func (r *Repository) deleteTag(tag string) error {
	ref, err := ociname.NewTag(fmt.Sprintf("%s:%s", r.Repository, tag))
	if err != nil {
		return fmt.Errorf("failed to parse reference: %w", err)
	}
	desc, err := remote.Head(ref, r.remoteOpts...)
	if err != nil {
		return fmt.Errorf("failed to fetch head for reference: %w", err)
	}

	deleteRef, err := parseReference(desc.Digest.String(), r)
	if err != nil {
		return fmt.Errorf("failed to construct reference for calculated digest: %w", err)
	}

	if err := remote.Delete(deleteRef, r.remoteOpts...); err != nil {
		return fmt.Errorf("failed to delete ref '%s': %w", ref, err)
	}

	return nil
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
func (r *Repository) PushStreamingImage(
	reference string,
	reader io.ReadCloser,
	mediaType string,
	annotations map[string]string,
) (*v1.Manifest, error) {
	ref, err := parseReference(reference, r)
	if err != nil {
		return nil, fmt.Errorf("failed to parse reference: %w", err)
	}
	image, err := computeStreamImage(reader, mediaType)
	if err != nil {
		return nil, fmt.Errorf("failed to compute image: %w", err)
	}
	if len(annotations) > 0 {
		i, ok := mutate.Annotations(image, annotations).(v1.Image)
		if !ok {
			return nil, fmt.Errorf("returned object was not an Image")
		}

		image = i
	}

	// These MediaTypes are required to create a Helm compliant OCI repository.
	if mediaType == registry.ChartLayerMediaType {
		image = mutate.ConfigMediaType(image, registry.ConfigMediaType)
		image = mutate.MediaType(image, ocispec.MediaTypeImageManifest)
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

	// check if the manifest annotations match the given filters
	var annotations map[string]string
	if len(filters) > 0 {
		// get descriptor from manifest
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
	return mutate.AppendLayers(empty.Image, computeStreamBlob(reader, mediaType))
}

func computeStreamBlob(reader io.ReadCloser, mediaType string) v1.Layer {
	t := types.MediaType(mediaType)
	if t == "" {
		t = types.OCILayer
	}

	return stream.NewLayer(reader, stream.WithMediaType(t))
}

func getDescriptor(manifest []byte) (*v1.Descriptor, error) {
	desc := &v1.Descriptor{}
	if err := json.Unmarshal(manifest, desc); err != nil {
		return nil, fmt.Errorf("failed to unmarshal manifest: %w", err)
	}

	return desc, nil
}
