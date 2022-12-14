// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package oci

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/stream"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	types2 "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	ocmclient "github.com/open-component-model/ocm-controller/pkg/ocm"
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

// Client defines OCI functionality.
//
//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . Client
type Client interface {
	PushResource(ctx context.Context, opts ResourceOptions) error
	FetchAndCacheResource(ctx context.Context, opts ResourceOptions) (io.ReadCloser, error)
}

// OCIClient abstracts the use of oci methods.
type OCIClient struct {
	ocmClient  ocmclient.FetchVerifier
	kubeClient client.Client
	ociAddress string
	scheme     *runtime.Scheme
}

// NewClient creates a new OCI Client.
func NewClient(ocmClient ocmclient.FetchVerifier, kubeClient client.Client, ociAddress string, scheme *runtime.Scheme) *OCIClient {
	return &OCIClient{
		ocmClient:  ocmClient,
		kubeClient: kubeClient,
		ociAddress: ociAddress,
		scheme:     scheme,
	}
}

// Repository is a wrapper around go-container registry's name.Repository.
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

// PushResource takes a resource, reference path and identity information and caches a resource in the internal OCI registry.
func (c *OCIClient) PushResource(ctx context.Context, opts ResourceOptions) error {
	reader, err := c.ocmClient.GetResource(ctx, opts.ComponentVersion, opts.Resource)
	if err != nil {
		return fmt.Errorf("failed to get resource: %w", err)
	}
	defer reader.Close()

	if _, err := c.cacheResource(ctx, opts.ComponentVersion, opts.Resource, opts.Owner, opts.SnapshotName, reader); err != nil {
		return fmt.Errorf("failed to cache resource '%s': %w", opts.Resource.Name, err)
	}

	return nil
}

// FetchAndCacheResource fetches and then caches a resource. It will only fetch a resource if it doesn't already have a snapshot.
func (c *OCIClient) FetchAndCacheResource(ctx context.Context, opts ResourceOptions) (io.ReadCloser, error) {
	snapshotName := opts.SnapshotName
	if snapshotName == "" {
		snapshotName = generateSnapshotNameForResource(*opts.ComponentVersion, opts.Resource)
	}
	snapshot := &v1alpha1.Snapshot{}
	if err := c.kubeClient.Get(ctx, types2.NamespacedName{
		Name:      snapshotName,
		Namespace: opts.ComponentVersion.Namespace,
	}, snapshot); err != nil {
		if apierrors.IsNotFound(err) {
			return c.fetchAndCacheResource(ctx, opts.ComponentVersion, opts.Resource, opts.Owner, opts.SnapshotName)
		}
		return nil, fmt.Errorf("failed to fetch is snapshot exists: %w", err)
	}

	// TODO: Add snapshot status check.
	image := strings.TrimPrefix(snapshot.Status.RepositoryURL, "http://")
	image = strings.TrimPrefix(image, "https://")
	repo, err := NewRepository(image, WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}

	blob, err := repo.FetchBlob(snapshot.Status.Digest)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch blob: %w", err)
	}

	return blob, nil
}

// TODO: Only use this if a name is not provided.
func generateSnapshotNameForResource(cv v1alpha1.ComponentVersion, resource v1alpha1.ResourceRef) string {
	// TODO: This potentially is larger than 64.
	// This is deliberately not component name.
	normalize := func(s string) string {
		s = strings.ReplaceAll(s, ".", "-")
		s = strings.ReplaceAll(s, "/", "-")
		return s
	}
	return fmt.Sprintf("%s-%s-%s-%s", cv.Name, normalize(cv.Status.ReconciledVersion), resource.Name, normalize(resource.Version))
}

// cacheResource caches the resource in a snapshot and an OCI layer.
func (c *OCIClient) cacheResource(ctx context.Context, componentVersion *v1alpha1.ComponentVersion, resource v1alpha1.ResourceRef, owner metav1.Object, snapshotName string, reader io.ReadCloser) (*v1alpha1.Snapshot, error) {
	repositoryName := c.constructRepositoryName(*componentVersion, resource)
	repo, err := NewRepository(repositoryName, WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("failed create new repository: %w", err)
	}

	// TODO: add extra identity
	digest, err := repo.PushStreamingImage(resource.Version, reader, "", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to push image: %w", err)
	}
	if snapshotName == "" {
		snapshotName = generateSnapshotNameForResource(*componentVersion, resource)
	}
	// create/update the snapshot custom resource
	snapshotCR := &v1alpha1.Snapshot{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: componentVersion.GetNamespace(),
			Name:      snapshotName,
		},
	}

	_, err = controllerutil.CreateOrUpdate(ctx, c.kubeClient, snapshotCR, func() error {
		if snapshotCR.ObjectMeta.CreationTimestamp.IsZero() {
			if err := controllerutil.SetOwnerReference(owner, snapshotCR, c.scheme); err != nil {
				return fmt.Errorf("failed to set owner to snapshot object: %w", err)
			}
		}
		snapshotCR.Spec = v1alpha1.SnapshotSpec{
			Ref: strings.TrimPrefix(repositoryName, c.ociAddress+"/"),
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create or update component descriptor: %w", err)
	}

	newSnapshotCR := snapshotCR.DeepCopy()
	newSnapshotCR.Status.Digest = digest
	newSnapshotCR.Status.Tag = resource.Version
	if err := patchObject(ctx, c.kubeClient, snapshotCR, newSnapshotCR); err != nil {
		return nil, fmt.Errorf("failed to patch snapshot CR: %w", err)
	}

	return newSnapshotCR, nil
}

// fetchAndCacheResource takes a resource, creates a snapshot for it and pushes it into an OCI layer.
func (c *OCIClient) fetchAndCacheResource(ctx context.Context, componentVersion *v1alpha1.ComponentVersion, resource v1alpha1.ResourceRef, owner metav1.Object, snapshotName string) (io.ReadCloser, error) {
	reader, err := c.ocmClient.GetResource(ctx, componentVersion, resource)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch reader: %w", err)
	}
	defer reader.Close()

	snapshot, err := c.cacheResource(ctx, componentVersion, resource, owner, snapshotName, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to cache resource: %w", err)
	}

	repo, err := NewRepository(c.constructRepositoryName(*componentVersion, resource), WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("failed create new repository: %w", err)
	}

	blob, err := repo.FetchBlob(snapshot.Status.Digest)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch blob: %w", err)
	}
	return blob, nil
}

func (c *OCIClient) constructRepositoryName(componentVersion v1alpha1.ComponentVersion, resource v1alpha1.ResourceRef) string {
	return fmt.Sprintf(
		"%s/%s/%s",
		c.ociAddress,
		componentVersion.Name,
		resource.Name,
	)
}

func patchObject(ctx context.Context, client client.Client, oldObject, newObject client.Object) error {
	patchHelper, err := patch.NewHelper(oldObject, client)
	if err != nil {
		return fmt.Errorf("failed to create patch helper: %w", err)
	}
	if err := patchHelper.Patch(ctx, newObject); err != nil {
		return fmt.Errorf("failed to patch object: %w", err)
	}
	return nil
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
func (r *Repository) PushStreamingImage(reference string, reader io.ReadCloser, mediaType string, annotations map[string]string) (string, error) {
	ref, err := parseReference(reference, r)
	if err != nil {
		return "", fmt.Errorf("failed to parse reference: %w", err)
	}
	image, err := computeStreamImage(reader, mediaType)
	if err != nil {
		return "", fmt.Errorf("failed to compute image: %w", err)
	}
	if len(annotations) > 0 {
		image = mutate.Annotations(image, annotations).(v1.Image)
	}
	if err := r.pushImage(image, ref); err != nil {
		return "", fmt.Errorf("failed to push image: %w", err)
	}
	layers, err := image.Layers()
	if err != nil {
		return "", fmt.Errorf("failed to get layers: %w", err)
	}
	digest, err := layers[0].Digest()
	if err != nil {
		return "", fmt.Errorf("failed to calculate digest for image: %w", err)
	}
	return digest.String(), nil
}

// pushImage pushes an OCI image to the repository. It accepts a v1.RepositoryURL interface.
func (r *Repository) pushImage(image v1.Image, reference name.Reference) error {
	return remote.Write(reference, image, r.remoteOpts...)
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
