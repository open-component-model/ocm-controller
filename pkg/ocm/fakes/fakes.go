package fakes

import (
	"context"
	"io"

	"github.com/open-component-model/ocm/pkg/contexts/ocm"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	ocmctrl "github.com/open-component-model/ocm-controller/pkg/ocm"
)

// MockFetcher mocks OCM client. Sadly, no generated code can be used, because none of them understand
// not importing type aliased names that OCM uses. Meaning, external types request internally aliased
// resources and the mock does not compile.
// I.e.: counterfeiter: https://github.com/maxbrunsfeld/counterfeiter/issues/174
type MockFetcher struct {
	getResourceReader                   io.ReadCloser
	getResourceErr                      error
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

func (m *MockFetcher) GetResource(ctx context.Context, cv *v1alpha1.ComponentVersion, resource v1alpha1.ResourceRef) (io.ReadCloser, error) {
	m.getResourceCalledWith = append(m.getResourceCalledWith, []any{cv, resource})
	return m.getResourceReader, m.getResourceErr
}

func (m *MockFetcher) GetResourceReturns(reader io.ReadCloser, err error) {
	m.getResourceReader = reader
	m.getResourceErr = err
}

func (m *MockFetcher) GetResourceCallingArgumentsOnCall(i int) []any {
	return m.getResourceCalledWith[i]
}

func (m *MockFetcher) GetComponentVersion(ctx context.Context, obj *v1alpha1.ComponentVersion, name, version string) (ocm.ComponentVersionAccess, error) {
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

func (m *MockFetcher) VerifyComponent(ctx context.Context, obj *v1alpha1.ComponentVersion, version string) (bool, error) {
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

func (m *MockFetcher) GetLatestComponentVersion(ctx context.Context, obj *v1alpha1.ComponentVersion) (string, error) {
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

func (m *MockFetcher) ListComponentVersions(ocmCtx ocm.Context, obj *v1alpha1.ComponentVersion) ([]ocmctrl.Version, error) {
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
