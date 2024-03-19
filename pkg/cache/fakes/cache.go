// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package fakes

import (
	"context"
	"fmt"
	"io"

	"github.com/open-component-model/ocm-controller/pkg/cache"
)

// fetchDataByDigestReturnValues defines the values returned by fetchDataByDigest.
type fetchDataByDigestReturnValues struct {
	reader io.ReadCloser
	err    error
}

type FakeCache struct {
	isCachedBool                  bool
	isCachedErr                   error
	isCachedCalledWith            [][]any
	pushDataString                string
	pushDataSize                  int64
	pushDataErr                   error
	pushDataCalledWith            []PushDataArguments
	fetchDataByIdentityReader     io.ReadCloser
	fetchDataByIdentityDigest     string
	fetchDataByIdentityErr        error
	fetchDataByIdentityCalledWith [][]any
	fetchDataByDigestCallCount    int
	fetchDataByDigestReturns      map[int]fetchDataByDigestReturnValues
	fetchDataByDigestCalledWith   [][]any
	deleteDataErr                 error
	deleteDataCalledWith          [][]any
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

func (f *FakeCache) IsCachedWasNotCalled() bool {
	return len(f.isCachedCalledWith) == 0
}

func (f *FakeCache) PushData(ctx context.Context, data io.ReadCloser, mediaType, name, tag string) (string, int64, error) {
	content, err := io.ReadAll(data)
	if err != nil {
		return "", -1, fmt.Errorf("failed to read read closer: %w", err)
	}

	f.pushDataCalledWith = append(f.pushDataCalledWith, PushDataArguments{Content: string(content), Name: name, Version: tag})
	return f.pushDataString, f.pushDataSize, f.pushDataErr
}

func (f *FakeCache) PushDataReturns(digest string, err error) {
	f.pushDataString = digest
	f.pushDataErr = err
}

type PushDataArguments struct {
	Name    string
	Version string
	Content string
}

func (f *FakeCache) PushDataCallingArgumentsOnCall(i int) PushDataArguments {
	return f.pushDataCalledWith[i]
}

func (f *FakeCache) PushDataWasNotCalled() bool {
	return len(f.pushDataCalledWith) == 0
}

func (f *FakeCache) FetchDataByIdentity(ctx context.Context, name, tag string) (io.ReadCloser, string, int64, error) {
	f.fetchDataByIdentityCalledWith = append(f.fetchDataByIdentityCalledWith, []any{name, tag})
	return f.fetchDataByIdentityReader, f.fetchDataByIdentityDigest, -1, f.fetchDataByIdentityErr
}

func (f *FakeCache) FetchDataByIdentityReturns(reader io.ReadCloser, err error) {
	f.fetchDataByIdentityReader = reader
	f.fetchDataByIdentityErr = err
}

func (f *FakeCache) FetchDataByIdentityCallingArgumentsOnCall(i int) []any {
	return f.fetchDataByIdentityCalledWith[i]
}

func (f *FakeCache) FetchDataByIdentityWasNotCalled() bool {
	return len(f.fetchDataByIdentityCalledWith) == 0
}

func (f *FakeCache) FetchDataByDigest(ctx context.Context, name, digest string) (io.ReadCloser, error) {
	if _, ok := f.fetchDataByDigestReturns[f.fetchDataByDigestCallCount]; !ok {
		return nil, fmt.Errorf("unexpected number of calls; not enough return values have been configured; call count %d", f.fetchDataByDigestCallCount)
	}
	f.fetchDataByDigestCalledWith = append(f.fetchDataByDigestCalledWith, []any{name, digest})
	result := f.fetchDataByDigestReturns[f.fetchDataByDigestCallCount]
	f.fetchDataByDigestCallCount++
	return result.reader, result.err
}

func (f *FakeCache) FetchDataByDigestReturns(reader io.ReadCloser, err error) {
	if f.fetchDataByDigestReturns == nil {
		f.fetchDataByDigestReturns = make(map[int]fetchDataByDigestReturnValues)
	}
	f.fetchDataByDigestReturns[0] = fetchDataByDigestReturnValues{
		reader: reader,
		err:    err,
	}
}

func (f *FakeCache) FetchDataByDigestReturnsOnCall(n int, reader io.ReadCloser, err error) {
	if f.fetchDataByDigestReturns == nil {
		f.fetchDataByDigestReturns = make(map[int]fetchDataByDigestReturnValues)
	}
	f.fetchDataByDigestReturns[n] = fetchDataByDigestReturnValues{
		reader: reader,
		err:    err,
	}
}

func (f *FakeCache) FetchDataByDigestCallingArgumentsOnCall(i int) []any {
	return f.fetchDataByDigestCalledWith[i]
}

func (f *FakeCache) FetchDataByDigestWasNotCalled() bool {
	return len(f.fetchDataByDigestCalledWith) == 0
}

func (f *FakeCache) DeleteData(ctx context.Context, name, digest string) error {
	f.deleteDataCalledWith = append(f.deleteDataCalledWith, []any{name, digest})
	return f.deleteDataErr
}

func (f *FakeCache) DeleteDataReturns(err error) {
	f.deleteDataErr = err
}

func (f *FakeCache) DeleteDataCallingArgumentsOnCall(i int) []any {
	return f.deleteDataCalledWith[i]
}

func (f *FakeCache) DeleteDataWasNotCalled() bool {
	return len(f.deleteDataCalledWith) == 0
}

var _ cache.Cache = &FakeCache{}
