package fakes

import (
	"context"
	"fmt"
	"io"

	"github.com/open-component-model/ocm-controller/pkg/cache"
)

type FakeCache struct {
	isCachedBool                  bool
	isCachedErr                   error
	isCachedCalledWith            [][]any
	pushDataString                string
	pushDataErr                   error
	pushDataCalledWith            [][]any
	fetchDataByIdentityReader     io.ReadCloser
	fetchDataByIdentityErr        error
	fetchDataByIdentityCalledWith [][]any
	fetchDataByDigestReader       io.ReadCloser
	fetchDataByDigestErr          error
	fetchDataByDigestCalledWith   [][]any
}

func (f *FakeCache) IsCached(ctx context.Context, name, tag string) (bool, error) {
	f.isCachedCalledWith = append(f.isCachedCalledWith, []any{name, tag})
	return f.isCachedBool, f.isCachedErr
}

func (f *FakeCache) IsCachedReturns(cached bool, err error) {
	f.isCachedBool = cached
	f.isCachedErr = err
}

func (f *FakeCache) IsCachedCallingArgumentsOnCall(i int) []any {
	return f.isCachedCalledWith[i]
}

func (f *FakeCache) PushData(ctx context.Context, data io.ReadCloser, name, tag string) (string, error) {
	content, err := io.ReadAll(data)
	if err != nil {
		return "", fmt.Errorf("failed to read read closer: %w", err)
	}
	f.pushDataCalledWith = append(f.pushDataCalledWith, []any{string(content), name, tag})
	return f.pushDataString, f.pushDataErr
}

func (f *FakeCache) PushDataReturns(digest string, err error) {
	f.pushDataString = digest
	f.pushDataErr = err
}

func (f *FakeCache) PushDataCallingArgumentsOnCall(i int) []any {
	return f.pushDataCalledWith[i]
}

func (f *FakeCache) FetchDataByIdentity(ctx context.Context, name, tag string) (io.ReadCloser, error) {
	f.fetchDataByIdentityCalledWith = append(f.fetchDataByIdentityCalledWith, []any{name, tag})
	return f.fetchDataByIdentityReader, f.fetchDataByIdentityErr
}

func (f *FakeCache) FetchDataByIdentityReturns(reader io.ReadCloser, err error) {
	f.fetchDataByIdentityReader = reader
	f.fetchDataByIdentityErr = err
}

func (f *FakeCache) FetchDataByIdentityCallingArgumentsOnCall(i int) []any {
	return f.fetchDataByIdentityCalledWith[i]
}

func (f *FakeCache) FetchDataByDigest(ctx context.Context, name, digest string) (io.ReadCloser, error) {
	f.fetchDataByDigestCalledWith = append(f.fetchDataByDigestCalledWith, []any{name, digest})
	return f.fetchDataByDigestReader, f.fetchDataByDigestErr
}

func (f *FakeCache) FetchDataByDigestReturns(reader io.ReadCloser, err error) {
	f.fetchDataByDigestReader = reader
	f.fetchDataByDigestErr = err
}

func (f *FakeCache) FetchDataByDigestCallingArgumentsOnCall(i int) []any {
	return f.fetchDataByDigestCalledWith[i]
}

var _ cache.Cache = &FakeCache{}
