package fakes

import (
	"context"
	"io"

	"github.com/open-component-model/ocm-controller/pkg/cache"
)

type FakeCache struct {
	IsCachedBool              bool
	IsCachedErr               error
	PushDataString            string
	PushDataErr               error
	FetchDataByIdentityReader io.ReadCloser
	FetchDataByIdentityErr    error
	FetchDataByDigestReader   io.ReadCloser
	FetchDataByDigestErr      error
}

func (f *FakeCache) IsCached(ctx context.Context, name, tag string) (bool, error) {
	return f.IsCachedBool, f.IsCachedErr
}

func (f *FakeCache) PushData(ctx context.Context, data io.ReadCloser, name, tag string) (string, error) {
	return f.PushDataString, f.PushDataErr
}

func (f *FakeCache) FetchDataByIdentity(ctx context.Context, name, tag string) (io.ReadCloser, error) {
	return f.FetchDataByIdentityReader, f.FetchDataByIdentityErr
}

func (f *FakeCache) FetchDataByDigest(ctx context.Context, name, digest string) (io.ReadCloser, error) {
	return f.FetchDataByDigestReader, f.FetchDataByDigestErr
}

var _ cache.Cache = &FakeCache{}
