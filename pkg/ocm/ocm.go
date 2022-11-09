package ocm

import (
	"context"
	"fmt"

	csdk "github.com/open-component-model/ocm-controllers-sdk"
	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
)

// Fetcher gets an OCM component version based on a k8s component version.
type Fetcher interface {
	GetComponentVersion(ctx context.Context, obj *v1alpha1.ComponentVersion, name, version string) (ocm.ComponentVersionAccess, error)
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
