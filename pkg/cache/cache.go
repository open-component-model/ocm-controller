package cache

import (
	"context"
	"io"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
)

const (
	ComponentNameKey    = "component-name"
	ComponentVersionKey = "component-version"
	ResourceNameKey     = "resource-name"
	ResourceVersionKey  = "resource-version"
)

// Cache defines capabilities for a cache whatever the backing medium might be.
type Cache interface {
	IsCached(ctx context.Context, identity v1alpha1.Identity, tag string) (bool, error)
	PushData(ctx context.Context, data io.ReadCloser, identity v1alpha1.Identity, tag string) (string, error)
	FetchDataByIdentity(ctx context.Context, identifier v1alpha1.Identity, tag string) (io.ReadCloser, error)
	FetchDataByDigest(ctx context.Context, identity v1alpha1.Identity, digest string) (io.ReadCloser, error)
}
