package cache

import (
	"context"
	"io"
)

// Cache defines capabilities for a cache whatever the backing medium might be.
type Cache interface {
	IsCached(ctx context.Context, name, tag string) (bool, error)
	PushData(ctx context.Context, data io.ReadCloser, name, tag string) (string, error)
	FetchDataByIdentity(ctx context.Context, name, tag string) (io.ReadCloser, error)
	FetchDataByDigest(ctx context.Context, name, digest string) (io.ReadCloser, error)
}
