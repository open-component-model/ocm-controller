package ocm

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/containers/image/v5/pkg/compression"
	"github.com/go-logr/logr"
	"github.com/mandelsoft/vfs/pkg/memoryfs"
	"github.com/mandelsoft/vfs/pkg/vfs"
	"github.com/mitchellh/hashstructure/v2"
	"helm.sh/helm/v3/pkg/registry"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"ocm.software/ocm/api/credentials/extensions/repositories/dockerconfig"
	"ocm.software/ocm/api/ocm"
	ocmmetav1 "ocm.software/ocm/api/ocm/compdesc/meta/v1"
	"ocm.software/ocm/api/ocm/extensions/attrs/signingattr"
	"ocm.software/ocm/api/ocm/extensions/download"
	"ocm.software/ocm/api/ocm/extensions/repositories/ocireg"
	"ocm.software/ocm/api/ocm/resolvers"
	"ocm.software/ocm/api/ocm/resourcerefs"
	"ocm.software/ocm/api/ocm/tools/signing"
	"ocm.software/ocm/api/ocm/tools/transfer"
	"ocm.software/ocm/api/ocm/tools/transfer/transferhandler/standard"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/pkg/cache"
	"github.com/open-component-model/ocm-controller/pkg/component"
)

const dockerConfigKey = ".dockerconfigjson"

// Contract defines a subset of capabilities from the OCM library.
type Contract interface {
	CreateAuthenticatedOCMContext(ctx context.Context, obj *v1alpha1.ComponentVersion) (ocm.Context, error)
	GetResource(
		ctx context.Context,
		octx ocm.Context,
		cv *v1alpha1.ComponentVersion,
		resource *v1alpha1.ResourceReference,
	) (io.ReadCloser, string, int64, error)
	GetComponentVersion(
		ctx context.Context,
		octx ocm.Context,
		repositoryURL, name, version string,
	) (ocm.ComponentVersionAccess, error)
	GetLatestValidComponentVersion(ctx context.Context, octx ocm.Context, obj *v1alpha1.ComponentVersion) (string, error)
	ListComponentVersions(ctx context.Context, logger logr.Logger, octx ocm.Context, obj *v1alpha1.ComponentVersion) ([]Version, error)
	VerifyComponent(ctx context.Context, octx ocm.Context, obj *v1alpha1.ComponentVersion, version string) (bool, error)
	TransferComponent(
		octx ocm.Context,
		obj *v1alpha1.ComponentVersion,
		sourceComponentVersion ocm.ComponentVersionAccess,
	) error
}

// Client implements the OCM fetcher interface.
type Client struct {
	client client.Client
	cache  cache.Cache
}

var _ Contract = &Client{}

// NewClient creates a new fetcher Client using the provided k8s client.
func NewClient(client client.Client, cache cache.Cache) *Client {
	return &Client{
		client: client,
		cache:  cache,
	}
}

func (c *Client) CreateAuthenticatedOCMContext(ctx context.Context, obj *v1alpha1.ComponentVersion) (ocm.Context, error) {
	octx := ocm.New()

	if obj.Spec.ServiceAccountName != "" {
		if err := c.configureServiceAccountAccess(ctx, octx, obj.Spec.ServiceAccountName, obj.Namespace); err != nil {
			return nil, fmt.Errorf("failed to configure service account access: %w", err)
		}
	}

	if err := c.configureAccessCredentials(ctx, octx, obj.Spec.Repository, obj.Namespace); err != nil {
		return nil, fmt.Errorf("failed to configure credentials for source: %w", err)
	}

	return octx, nil
}

// configureAccessCredentials configures access credentials if needed for a source/destination repository.
func (c *Client) configureAccessCredentials(
	ctx context.Context,
	ocmCtx ocm.Context,
	repository v1alpha1.Repository,
	namespace string,
) error {
	// If there are no credentials, this call is a no-op.
	if repository.SecretRef == nil {
		return nil
	}

	logger := log.FromContext(ctx)

	if err := ConfigureCredentials(ctx, ocmCtx, c.client, repository.URL, repository.SecretRef.Name, namespace); err != nil {
		logger.V(v1alpha1.LevelDebug).Error(err, "failed to find credentials")

		// we don't ignore not found errors
		return fmt.Errorf("failed to configure credentials for component: %w", err)
	}

	logger.V(v1alpha1.LevelDebug).Info("credentials configured")

	return nil
}

func (c *Client) configureServiceAccountAccess(
	ctx context.Context,
	octx ocm.Context,
	serviceAccountName, namespace string,
) error {
	logger := log.FromContext(ctx)

	logger.V(v1alpha1.LevelDebug).Info("configuring service account credentials")
	account := &corev1.ServiceAccount{}
	if err := c.client.Get(ctx, types.NamespacedName{
		Name:      serviceAccountName,
		Namespace: namespace,
	}, account); err != nil {
		return fmt.Errorf("failed to fetch service account: %w", err)
	}

	logger.V(v1alpha1.LevelDebug).Info("got service account", "name", account.GetName())

	for _, imagePullSecret := range account.ImagePullSecrets {
		secret := &corev1.Secret{}

		if err := c.client.Get(ctx, types.NamespacedName{
			Name:      imagePullSecret.Name,
			Namespace: namespace,
		}, secret); err != nil {
			return fmt.Errorf("failed to get image pull secret: %w", err)
		}

		data, ok := secret.Data[dockerConfigKey]
		if !ok {
			return fmt.Errorf("failed to find .dockerconfigjson in secret %s", secret.Name)
		}

		repository := dockerconfig.NewRepositorySpecForConfig(data, true)

		if _, err := octx.CredentialsContext().RepositoryForSpec(repository); err != nil {
			return fmt.Errorf("failed to configure credentials for repository: %w", err)
		}
	}

	return nil
}

// GetResource returns a reader for the resource data. It is the responsibility of the caller to close the reader.
func (c *Client) GetResource(
	ctx context.Context,
	octx ocm.Context,
	cv *v1alpha1.ComponentVersion,
	resource *v1alpha1.ResourceReference,
) (io.ReadCloser, string, int64, error) {
	cd, err := component.GetComponentDescriptor(ctx, c.client, resource.ReferencePath, cv.Status.ComponentDescriptor)
	if err != nil {
		return nil, "", -1, fmt.Errorf("failed to find component descriptor for reference: %w", err)
	}

	if cd == nil {
		return nil, "", -1, fmt.Errorf(
			"component descriptor not found for reference path: %+v",
			resource.ReferencePath,
		)
	}

	// we default to the latest component version that this resource belongs to if we don't have any versions for the resource.
	version := cd.Spec.Version
	if resource.ElementMeta.Version != "" {
		version = resource.ElementMeta.Version
	}

	identity := ocmmetav1.Identity{
		v1alpha1.ComponentNameKey:    cd.Name,
		v1alpha1.ComponentVersionKey: cd.Spec.Version,
		v1alpha1.ResourceNameKey:     resource.ElementMeta.Name,
		v1alpha1.ResourceVersionKey:  version,
	}

	// Add extra identity.
	for k, v := range resource.ElementMeta.ExtraIdentity {
		identity[k] = v
	}
	if len(resource.ReferencePath) > 0 {
		var builder strings.Builder
		for _, path := range resource.ReferencePath {
			builder.WriteString(path.String() + ":")
		}

		identity[v1alpha1.ResourceRefPath] = builder.String()
	}

	name, err := ConstructRepositoryName(identity)
	if err != nil {
		return nil, "", -1, fmt.Errorf("failed to construct name: %w", err)
	}

	cached, err := c.cache.IsCached(ctx, name, version)
	if err != nil {
		return nil, "", -1, fmt.Errorf("failed to check cache: %w", err)
	}
	if cached {
		return c.cache.FetchDataByIdentity(ctx, name, version)
	}

	cva, err := c.GetComponentVersion(ctx, octx, cv.GetRepositoryURL(), cv.Spec.Component, cv.Status.ReconciledVersion)
	if err != nil {
		return nil, "", -1, fmt.Errorf("failed to get component Version: %w", err)
	}
	defer func() {
		if cerr := cva.Close(); cerr != nil {
			err = errors.Join(err, cerr)
		}
	}()

	var identities []ocmmetav1.Identity
	identities = append(identities, resource.ReferencePath...)

	var extras []string
	for k, v := range resource.ExtraIdentity {
		extras = append(extras, k, v)
	}

	// NewIdentity creates name based identity, and extra identity is added as a key value pair.
	res, _, err := resourcerefs.ResolveResourceReference(
		cva,
		ocmmetav1.NewNestedResourceRef(ocmmetav1.NewIdentity(resource.Name, extras...), identities),
		cva.Repository(),
	)
	if err != nil {
		return nil, "", -1, fmt.Errorf(
			"failed to resolve reference path to resource: %s %w",
			resource.Name,
			err,
		)
	}

	reader, mediaType, err := c.fetchResourceReader(res, cva)
	if err != nil {
		return nil, "", -1, fmt.Errorf("failed to fetch reader for resource: %w", err)
	}
	defer func() {
		if cerr := reader.Close(); cerr != nil {
			err = errors.Join(err, cerr)
		}
	}()

	decompressedReader, _, err := compression.AutoDecompress(reader)
	if err != nil {
		return nil, "", -1, fmt.Errorf("failed to autodecompress content: %w", err)
	}
	defer func() {
		if cerr := decompressedReader.Close(); cerr != nil {
			err = errors.Join(err, cerr)
		}
	}()

	// We need to push the media type... And construct the right layers I guess.
	digest, size, err := c.cache.PushData(ctx, decompressedReader, mediaType, name, version)
	if err != nil {
		return nil, "", -1, fmt.Errorf("failed to cache blob: %w", err)
	}

	// re-fetch the resource to have a streamed reader available
	dataReader, err := c.cache.FetchDataByDigest(ctx, name, digest)
	if err != nil {
		return nil, "", -1, fmt.Errorf("failed to fetch resource: %w", err)
	}

	return dataReader, digest, size, nil
}

// GetComponentVersion returns a component Version. It's the caller's responsibility to clean it up and close the component Version once done with it.
func (c *Client) GetComponentVersion(
	_ context.Context,
	octx ocm.Context,
	repositoryURL, name, version string,
) (ocm.ComponentVersionAccess, error) {
	repoSpec := ocireg.NewRepositorySpec(repositoryURL, nil)
	if repoSpec == nil {
		return nil, fmt.Errorf("failed to construct repository spec for url: %s", repositoryURL)
	}

	repo, err := octx.RepositoryForSpec(repoSpec)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository for spec: %w", err)
	}
	defer repo.Close()

	cv, err := repo.LookupComponentVersion(name, version)
	if err != nil {
		return nil, fmt.Errorf("failed to look up component Version: %w", err)
	}

	return cv, nil
}

func (c *Client) VerifyComponent(
	ctx context.Context,
	octx ocm.Context,
	obj *v1alpha1.ComponentVersion,
	version string,
) (bool, error) {
	logger := log.FromContext(ctx)

	repoSpec := ocireg.NewRepositorySpec(obj.Spec.Repository.URL, nil)
	repo, err := octx.RepositoryForSpec(repoSpec)
	if err != nil {
		return false, fmt.Errorf("failed to get repository for spec: %w", err)
	}
	defer repo.Close()

	cv, err := repo.LookupComponentVersion(obj.Spec.Component, version)
	if err != nil {
		return false, fmt.Errorf("failed to look up component Version: %w", err)
	}
	defer cv.Close()

	resolver := resolvers.NewCompoundResolver(repo)

	for _, signature := range obj.Spec.Verify {
		var (
			cert []byte
			err  error
		)

		if signature.PublicKey.Value != "" {
			cert, err = signature.PublicKey.DecodePublicValue()
		} else {
			if signature.PublicKey.SecretRef == nil {
				return false, fmt.Errorf("kubernetes secret reference not provided")
			}

			cert, err = c.getPublicKey(
				ctx,
				obj.Namespace,
				signature.PublicKey.SecretRef.Name,
				signature.Name,
			)
		}

		if err != nil {
			return false, fmt.Errorf("failed to get public key for verification: %w", err)
		}

		opts := signing.NewOptions(
			signing.Resolver(resolver),
			signing.PublicKey(signature.Name, cert),
			signing.VerifyDigests(),
			signing.VerifySignature(signature.Name),
		)

		get := signingattr.Get(octx)
		if err := opts.Complete(get); err != nil {
			return false, fmt.Errorf("failed to complete signature check: %w", err)
		}

		dig, err := signing.Apply(nil, nil, cv, opts)
		if err != nil {
			return false, fmt.Errorf("failed to apply signing while verifying component: %w", err)
		}

		var value string
		for _, s := range cv.GetDescriptor().Signatures {
			if s.Name == signature.Name {
				value = s.Digest.Value

				break
			}
		}

		if value == "" {
			return false, fmt.Errorf(
				"signature with name '%s' not found in the list of provided ocm signatures",
				signature.Name,
			)
		}

		if dig.Value != value {
			return false, fmt.Errorf("%s signature did not match key value", signature.Name)
		}

		logger.Info("component verified", "signature", signature.Name)
	}

	return true, nil
}

func (c *Client) getPublicKey(
	ctx context.Context,
	namespace, name, signature string,
) ([]byte, error) {
	var secret corev1.Secret
	secretKey := client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}
	if err := c.client.Get(ctx, secretKey, &secret); err != nil {
		return nil, err
	}

	for key, value := range secret.Data {
		if key == signature {
			return value, nil
		}
	}

	return nil, errors.New("public key not found")
}

// GetLatestValidComponentVersion gets the latest version that still matches the constraint.
func (c *Client) GetLatestValidComponentVersion(
	ctx context.Context,
	octx ocm.Context,
	obj *v1alpha1.ComponentVersion,
) (string, error) {
	logger := log.FromContext(ctx)

	versions, err := c.ListComponentVersions(ctx, logger, octx, obj)
	if err != nil {
		return "", fmt.Errorf("failed to get component versions: %w", err)
	}

	if len(versions) == 0 {
		return "", fmt.Errorf("no versions found for component '%s'", obj.Spec.Component)
	}

	sort.SliceStable(versions, func(i, j int) bool {
		return versions[i].Semver.GreaterThan(versions[j].Semver)
	})

	constraint, err := semver.NewConstraint(obj.Spec.Version.Semver)
	if err != nil {
		return "", fmt.Errorf("failed to parse constraint version: %w", err)
	}

	for _, v := range versions {
		if valid, _ := constraint.Validate(v.Semver); valid {
			if len(obj.Spec.Verify) > 0 {
				if _, err := c.VerifyComponent(ctx, octx, obj, v.Version); err != nil {
					logger.Error(err, "ignoring version as it failed verification", "version", v.Version, "component", obj.Spec.Component)

					continue
				}
			}

			return v.Version, nil
		}
	}

	return "", fmt.Errorf("no matching versions found for constraint '%s'", obj.Spec.Version.Semver)
}

// Version has two values to be able to sort a list but still return the actual Version.
// The Version might contain a `v`.
type Version struct {
	Semver  *semver.Version
	Version string
}

func (c *Client) ListComponentVersions(
	_ context.Context,
	logger logr.Logger,
	octx ocm.Context,
	obj *v1alpha1.ComponentVersion,
) ([]Version, error) {
	repoSpec := ocireg.NewRepositorySpec(obj.Spec.Repository.URL, nil)

	repo, err := octx.RepositoryForSpec(repoSpec)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository for spec: %w", err)
	}
	defer repo.Close()

	// get the component Version
	cv, err := repo.LookupComponent(obj.Spec.Component)
	if err != nil {
		return nil, fmt.Errorf("component error: %w", err)
	}
	defer cv.Close()

	versions, err := cv.ListVersions()
	if err != nil {
		return nil, fmt.Errorf("failed to list versions for component: %w", err)
	}

	var result []Version
	for _, v := range versions {
		parsed, err := semver.NewVersion(v)
		if err != nil {
			logger.Error(err, "ignoring version as it was invalid semver", "version", v)
			// ignore versions that are invalid semver.
			continue
		}
		result = append(result, Version{
			Semver:  parsed,
			Version: v,
		})
	}

	return result, nil
}

func (c *Client) TransferComponent(
	octx ocm.Context,
	obj *v1alpha1.ComponentVersion,
	sourceComponentVersion ocm.ComponentVersionAccess,
) error {
	sourceRepoSpec := ocireg.NewRepositorySpec(obj.Spec.Repository.URL, nil)
	source, err := octx.RepositoryForSpec(sourceRepoSpec)
	if err != nil {
		return fmt.Errorf("failed to get source repo: %w", err)
	}
	defer source.Close()

	targetRepoSpec := ocireg.NewRepositorySpec(obj.Spec.Destination.URL, nil)
	target, err := octx.RepositoryForSpec(targetRepoSpec)
	if err != nil {
		return fmt.Errorf("failed to get target repo: %w", err)
	}
	defer target.Close()

	handler, err := standard.New(
		standard.Recursive(true),
		standard.ResourcesByValue(true),
		standard.Overwrite(true),
		standard.Resolver(source),
		standard.Resolver(target),
	)
	if err != nil {
		return fmt.Errorf("failed to construct target handler: %w", err)
	}

	if err := transfer.TransferVersion(
		nil,
		transfer.TransportClosure{},
		sourceComponentVersion,
		target,
		handler,
	); err != nil {
		return fmt.Errorf("failed to transfer version to destination repository: %w", err)
	}

	return nil
}

// We add this decision because OCM is storing the Helm artifact as an ociArtifact at the
// time of this writing. This means, when fetching the resource via the normal route
// it will return an OCI blob instead of the actual helm chart content.
// Because of that, we need to create our own downloader when we are dealing with
// helm charts.
func (c *Client) fetchResourceReader(res ocm.ResourceAccess, cva ocm.ComponentVersionAccess) (_ io.ReadCloser, _ string, err error) {
	if res.Meta().Type == "helmChart" {
		return c.fetchHelmChartResource(res, cva, err)
	}

	// use the plain resource reader
	access, err := res.AccessMethod()
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch access spec: %w", err)
	}

	reader, err := access.Reader()
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch reader: %w", err)
	}

	// Ignore the media type as we set it to a default in OCI package
	return reader, "", nil
}

func (c *Client) fetchHelmChartResource(res ocm.ResourceAccess, cva ocm.ComponentVersionAccess, err error) (io.ReadCloser, string, error) {
	vf := vfs.New(memoryfs.New())
	defer func() {
		if rerr := vf.RemoveAll("downloaded"); rerr != nil {
			// ignore not exist errors that vfs implementation can throw sometimes.
			if !errors.Is(rerr, os.ErrNotExist) {
				err = errors.Join(err, rerr)
			}
		}
	}()

	d := download.For(cva.GetContext())
	// Note that helm downloader does _NOT_ return the path element of the Downloader's output.
	_, chart, err := d.Download(nil, res, "downloaded", vf)
	if err != nil {
		return nil, "", fmt.Errorf("failed to download helm chart content: %w", err)
	}

	content, rerr := vf.ReadFile(chart)
	if rerr != nil {
		return nil, "", fmt.Errorf("failed to find the downloaded file: %w", rerr)
	}
	reader := io.NopCloser(bytes.NewBuffer(content))

	return reader, registry.ChartLayerMediaType, nil
}

// ConstructRepositoryName hashes the name and passes it back.
func ConstructRepositoryName(identity ocmmetav1.Identity) (string, error) {
	repositoryName, err := HashIdentity(identity)
	if err != nil {
		return "", fmt.Errorf("failed to create hash for identity: %w", err)
	}

	return repositoryName, nil
}

// HashIdentity returns the string hash of an ocm identity.
func HashIdentity(id ocmmetav1.Identity) (string, error) {
	hash, err := hashstructure.Hash(id, hashstructure.FormatV2, nil)
	if err != nil {
		return "", fmt.Errorf("failed to hash identity: %w", err)
	}

	return fmt.Sprintf("sha-%d", hash), nil
}
