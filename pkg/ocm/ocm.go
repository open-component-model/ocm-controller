package ocm

import (
	"context"
	"errors"
	"fmt"

	csdk "github.com/open-component-model/ocm-controllers-sdk"
	"github.com/open-component-model/ocm/pkg/contexts/oci/repositories/ocireg"
	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/attrs/signingattr"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/repositories/genericocireg"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/signing"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
)

// Verifier takes a Component and runs OCM verification on it.
type Verifier interface {
	VerifyComponent(ctx context.Context, obj *v1alpha1.ComponentVersion) (bool, error)
}

// Fetcher gets an OCM component version based on a k8s component version.
type Fetcher interface {
	GetComponentVersion(ctx context.Context, obj *v1alpha1.ComponentVersion, name, version string) (ocm.ComponentVersionAccess, error)
}

// FetchVerifier can fetch and verify components.
type FetchVerifier interface {
	Verifier
	Fetcher
}

// Client implements the OCM fetcher interface.
type Client struct {
	client client.Client
}

// NewClient creates a new fetcher Client using the provided k8s client.
func NewClient(client client.Client) *Client {
	return &Client{
		client: client,
	}
}

func (c *Client) GetComponentVersion(ctx context.Context, obj *v1alpha1.ComponentVersion, name, version string) (ocm.ComponentVersionAccess, error) {
	log := log.FromContext(ctx)
	session := ocm.NewSession(nil)
	defer session.Close()

	ocmCtx := ocm.ForContext(ctx)
	// configure registry credentials
	if err := csdk.ConfigureCredentials(ctx, ocmCtx, c.client, obj.Spec.Repository.URL, obj.Spec.Repository.SecretRef.Name, obj.Namespace); err != nil {
		log.V(4).Error(err, "failed to find credentials")
		// ignore not found errors for now
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to configure credentials for component: %w", err)
		}
	}

	// get component version
	cv, err := csdk.GetComponentVersion(ocmCtx, session, obj.Spec.Repository.URL, name, version)
	if err != nil {
		return nil, fmt.Errorf("failed to get component version: %w", err)
	}

	return cv, nil
}

func (c *Client) VerifyComponent(ctx context.Context, obj *v1alpha1.ComponentVersion) (bool, error) {
	session := ocm.NewSession(nil)
	defer session.Close()

	ocmCtx := ocm.ForContext(ctx)

	if err := csdk.ConfigureCredentials(ctx, ocmCtx, c.client, obj.Spec.Repository.URL, obj.Spec.Verify.SecretRef.Name, obj.Namespace); err != nil {
		return false, err
	}

	repoSpec := genericocireg.NewRepositorySpec(ocireg.NewRepositorySpec(obj.Spec.Repository.URL), nil)
	repo, err := session.LookupRepository(ocmCtx, repoSpec)
	if err != nil {
		return false, fmt.Errorf("repo error: %w", err)
	}

	resolver := ocm.NewCompoundResolver(repo)

	cv, err := session.LookupComponentVersion(repo, obj.Spec.Name, obj.Spec.Version)
	if err != nil {
		return false, fmt.Errorf("component error: %w", err)
	}

	cert, err := c.getPublicKey(ctx, obj)
	if err != nil {
		return false, fmt.Errorf("verify error: %w", err)
	}

	opts := signing.NewOptions(
		signing.VerifySignature(obj.Spec.Verify.Signature),
		signing.Resolver(resolver),
		signing.VerifyDigests(),
		signing.PublicKey(obj.Spec.Verify.Signature, cert),
	)

	if err := opts.Complete(signingattr.Get(ocmCtx)); err != nil {
		return false, fmt.Errorf("verify error: %w", err)
	}

	dig, err := signing.Apply(nil, nil, cv, opts)
	if err != nil {
		return false, err
	}

	return dig.Value == cv.GetDescriptor().Signatures[0].Digest.Value, nil
}

func (c *Client) getPublicKey(ctx context.Context, obj *v1alpha1.ComponentVersion) ([]byte, error) {
	var secret corev1.Secret
	secretKey := client.ObjectKey{
		Namespace: obj.GetNamespace(),
		Name:      obj.Spec.Verify.SecretRef.Name,
	}
	if err := c.client.Get(ctx, secretKey, &secret); err != nil {
		return nil, err
	}

	for key, value := range secret.Data {
		if key == obj.Spec.Verify.Signature {
			return value, nil
		}
	}

	return nil, errors.New("public key not found")
}
