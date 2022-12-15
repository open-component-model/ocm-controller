package fakes

import (
	"context"
	"io"
	"testing"

	"github.com/open-component-model/ocm/pkg/contexts/ocm"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	ocmctrl "github.com/open-component-model/ocm-controller/pkg/ocm"
)

// MockFetcher mocks OCM client. Sadly, no generated code can be used, because none of them understand
// not importing type aliased names that OCM uses. Meaning, external types request internally aliased
// resources and the mock does not compile.
// I.e.: counterfeiter: https://github.com/maxbrunsfeld/counterfeiter/issues/174
type MockFetcher struct {
	GetComponentErr           error
	VerifyErr                 error
	GetVersionErr             error
	ComponentVersionAccessMap map[string]ocm.ComponentVersionAccess
	T                         *testing.T
	Verified                  bool
	LatestVersion             string
	FetchedResource           io.ReadCloser
	Digest                    string
}

func (m *MockFetcher) GetResource(ctx context.Context, cv *v1alpha1.ComponentVersion, resource v1alpha1.ResourceRef) (io.ReadCloser, error) {
	return m.FetchedResource, nil
}

func (m *MockFetcher) GetComponentVersion(ctx context.Context, obj *v1alpha1.ComponentVersion, name, version string) (ocm.ComponentVersionAccess, error) {
	m.T.Logf("called GetComponentVersion with name %s and version %s", name, version)
	return m.ComponentVersionAccessMap[name], m.GetComponentErr
}

func (m *MockFetcher) VerifyComponent(ctx context.Context, obj *v1alpha1.ComponentVersion, version string) (bool, error) {
	return m.Verified, m.VerifyErr
}

func (m *MockFetcher) GetLatestComponentVersion(ctx context.Context, obj *v1alpha1.ComponentVersion) (string, error) {
	return m.LatestVersion, m.GetVersionErr
}

func (m *MockFetcher) ListComponentVersions(ocmCtx ocm.Context, obj *v1alpha1.ComponentVersion) ([]ocmctrl.Version, error) {
	return []ocmctrl.Version{}, m.GetVersionErr
}
