// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package fakes

import (
	"context"
	"fmt"
	"io"

	"github.com/go-logr/logr"
	"github.com/open-component-model/ocm/pkg/contexts/ocm"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	ocmctrl "github.com/open-component-model/ocm-controller/pkg/ocm"
)

// getResourceReturnValues defines the return values of the GetResource function.
type getResourceReturnValues struct {
	reader io.ReadCloser
	digest string
	err    error
	size   int64
}

// MockFetcher mocks OCM client. Sadly, no generated code can be used, because none of them understand
// not importing type aliased names that OCM uses. Meaning, external types request internally aliased
// resources and the mock does not compile.
// I.e.: counterfeiter: https://github.com/maxbrunsfeld/counterfeiter/issues/174
type MockFetcher struct {
	getResourceCallCount                int
	getResourceReturns                  map[int]getResourceReturnValues
	getResourceCalledWith               [][]any
	getComponentVersionMap              map[string]ocm.ComponentVersionAccess
	getComponentVersionErr              error
	getComponentVersionCalledWith       [][]any
	verifyComponentErr                  error
	verifyComponentVerified             bool
	verifyComponentCalledWith           [][]any
	getLatestComponentVersionVersion    string
	getLatestComponentVersionErr        error
	getLatestComponentVersionCalledWith [][]any
	listComponentVersionsVersions       []ocmctrl.Version
	listComponentVersionsErr            error
	listComponentVersionsCalledWith     [][]any
}

var _ ocmctrl.Contract = &MockFetcher{}

func (m *MockFetcher) CreateAuthenticatedOCMContext(ctx context.Context, obj *v1alpha1.ComponentVersion) (ocm.Context, error) {
	return ocm.New(), nil
}

func (m *MockFetcher) GetResource(ctx context.Context, octx ocm.Context, cv *v1alpha1.ComponentVersion, resource *v1alpha1.ResourceReference) (io.ReadCloser, string, int64, error) {
	if _, ok := m.getResourceReturns[m.getResourceCallCount]; !ok {
		return nil, "", -1, fmt.Errorf("unexpected number of calls; not enough return values have been configured; call count %d", m.getResourceCallCount)
	}
	m.getResourceCalledWith = append(m.getResourceCalledWith, []any{cv, resource})
	result := m.getResourceReturns[m.getResourceCallCount]
	m.getResourceCallCount++
	return result.reader, result.digest, result.size, result.err
}

func (m *MockFetcher) GetResourceReturns(reader io.ReadCloser, digest string, err error) {
	if m.getResourceReturns == nil {
		m.getResourceReturns = make(map[int]getResourceReturnValues)
	}
	m.getResourceReturns[0] = getResourceReturnValues{
		reader: reader,
		digest: digest,
		err:    err,
	}
}

func (m *MockFetcher) GetResourceReturnsOnCall(n int, reader io.ReadCloser, err error) {
	if m.getResourceReturns == nil {
		m.getResourceReturns = make(map[int]getResourceReturnValues, 0)
	}
	m.getResourceReturns[n] = getResourceReturnValues{
		reader: reader,
		err:    err,
	}
}

func (m *MockFetcher) GetResourceCallingArgumentsOnCall(i int) []any {
	return m.getResourceCalledWith[i]
}

func (m *MockFetcher) GetResourceWasNotCalled() bool {
	return len(m.getResourceCalledWith) == 0
}

func (m *MockFetcher) GetComponentVersion(ctx context.Context, octx ocm.Context, obj *v1alpha1.ComponentVersion, name, version string) (ocm.ComponentVersionAccess, error) {
	m.getComponentVersionCalledWith = append(m.getComponentVersionCalledWith, []any{obj, name, version})
	return m.getComponentVersionMap[name], m.getComponentVersionErr
}

func (m *MockFetcher) GetComponentVersionReturnsForName(name string, cva ocm.ComponentVersionAccess, err error) {
	if m.getComponentVersionMap == nil {
		m.getComponentVersionMap = make(map[string]ocm.ComponentVersionAccess)
	}
	m.getComponentVersionMap[name] = cva
	m.getComponentVersionErr = err
}

func (m *MockFetcher) GetComponentVersionCallingArgumentsOnCall(i int) []any {
	return m.getComponentVersionCalledWith[i]
}

func (m *MockFetcher) GetComponentVersionWasNotCalled() bool {
	return len(m.getComponentVersionCalledWith) == 0
}

func (m *MockFetcher) VerifyComponent(ctx context.Context, octx ocm.Context, obj *v1alpha1.ComponentVersion, version string) (bool, error) {
	m.verifyComponentCalledWith = append(m.verifyComponentCalledWith, []any{obj, version})
	return m.verifyComponentVerified, m.verifyComponentErr
}

func (m *MockFetcher) VerifyComponentReturns(verified bool, err error) {
	m.verifyComponentVerified = verified
	m.verifyComponentErr = err
}

func (m *MockFetcher) VerifyComponentCallingArgumentsOnCall(i int) []any {
	return m.verifyComponentCalledWith[i]
}

func (m *MockFetcher) VerifyComponentWasNotCalled() bool {
	return len(m.verifyComponentCalledWith) == 0
}

func (m *MockFetcher) GetLatestValidComponentVersion(ctx context.Context, octx ocm.Context, obj *v1alpha1.ComponentVersion) (string, error) {
	m.getComponentVersionCalledWith = append(m.getComponentVersionCalledWith, []any{obj})
	return m.getLatestComponentVersionVersion, m.getLatestComponentVersionErr
}

func (m *MockFetcher) GetLatestComponentVersionReturns(version string, err error) {
	m.getLatestComponentVersionVersion = version
	m.getLatestComponentVersionErr = err
}

func (m *MockFetcher) GetLatestComponentVersionCallingArgumentsOnCall(i int) []any {
	return m.getLatestComponentVersionCalledWith[i]
}

func (m *MockFetcher) GetLatestComponentVersionWasNotCalled() bool {
	return len(m.getLatestComponentVersionCalledWith) == 0
}

func (m *MockFetcher) ListComponentVersions(logger logr.Logger, octx ocm.Context, obj *v1alpha1.ComponentVersion) ([]ocmctrl.Version, error) {
	m.listComponentVersionsCalledWith = append(m.listComponentVersionsCalledWith, []any{obj})
	return m.listComponentVersionsVersions, m.listComponentVersionsErr
}

func (m *MockFetcher) ListComponentVersionsReturns(versions []ocmctrl.Version, err error) {
	m.listComponentVersionsVersions = versions
	m.listComponentVersionsErr = err
}

func (m *MockFetcher) ListComponentVersionsCallingArgumentsOnCall(i int) []any {
	return m.listComponentVersionsCalledWith[i]
}

func (m *MockFetcher) ListComponentVersionsWasNotCalled() bool {
	return len(m.listComponentVersionsCalledWith) == 0
}
