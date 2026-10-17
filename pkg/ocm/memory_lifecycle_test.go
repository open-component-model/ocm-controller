package ocm

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/fluxcd/pkg/apis/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"ocm.software/ocm/api/credentials"
	ocmcore "ocm.software/ocm/api/ocm"
	ocmmeta "ocm.software/ocm/api/ocm/compdesc/meta/v1"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	cachefakes "github.com/open-component-model/ocm-controller/pkg/cache/fakes"
	fakeocm "github.com/open-component-model/ocm-controller/pkg/fakes"
)

type lifecycleComponent struct {
	ocmcore.ComponentVersionAccess
	resource   ocmcore.ResourceAccess
	duplicates []*lifecycleComponent
	closed     int
}

func (c *lifecycleComponent) Dup() (ocmcore.ComponentVersionAccess, error) {
	view := &lifecycleComponent{ComponentVersionAccess: c.ComponentVersionAccess, resource: c.resource}
	c.duplicates = append(c.duplicates, view)
	return view, nil
}
func (c *lifecycleComponent) GetResource(ocmmeta.Identity) (ocmcore.ResourceAccess, error) {
	return c.resource, nil
}
func (c *lifecycleComponent) Repository() ocmcore.Repository { return nil }
func (c *lifecycleComponent) Close() error                   { c.closed++; return nil }

type lifecycleRepository struct {
	ocmcore.Repository
	component *lifecycleComponent
}

func (r *lifecycleRepository) LookupComponentVersion(string, string) (ocmcore.ComponentVersionAccess, error) {
	return r.component, nil
}
func (r *lifecycleRepository) Close() error { return nil }

type lifecycleContext struct {
	ocmcore.Context
	repository *lifecycleRepository
}

func (c *lifecycleContext) RepositoryForSpec(ocmcore.RepositorySpec, ...credentials.CredentialsSource) (ocmcore.Repository, error) {
	return c.repository, nil
}

// This exercises GetResource against the real OCM resolver, with separate
// handles for each Dup so a missing Close is observable (the existing fake's
// Dup returns itself and Close is a no-op).
func TestGetResourceReleasesResolvedComponent(t *testing.T) {
	for _, pushFails := range []bool{false, true} {
		name := "success"
		if pushFails {
			name = "cache write failure"
		}
		t.Run(name, func(t *testing.T) {
			resource := &fakeocm.Resource[*ocmcore.ResourceMeta]{Name: "data", Data: []byte("payload"), Type: "ociBlob"}
			component := &lifecycleComponent{resource: resource}
			octx := &lifecycleContext{repository: &lifecycleRepository{component: component}}
			cd := &v1alpha1.ComponentDescriptor{ObjectMeta: metav1.ObjectMeta{Name: "descriptor", Namespace: "default"}}
			cache := &cachefakes.FakeCache{}
			cache.IsCachedReturns(false, nil)
			cache.FetchDataByDigestReturns(io.NopCloser(strings.NewReader("cached")), nil)
			var pushErr error
			if pushFails {
				pushErr = errors.New("cache write failed")
			}
			cache.PushDataReturns("digest", pushErr)
			c := NewClient(env.FakeKubeClient(WithObjects(cd)), cache)
			cv := &v1alpha1.ComponentVersion{
				ObjectMeta: metav1.ObjectMeta{Name: "component", Namespace: "default"},
				Spec:       v1alpha1.ComponentVersionSpec{Component: "example.org/component", Repository: v1alpha1.Repository{URL: "localhost"}},
				Status:     v1alpha1.ComponentVersionStatus{ReconciledVersion: "1.0.0", ComponentDescriptor: v1alpha1.Reference{ComponentDescriptorRef: meta.NamespacedObjectReference{Name: "descriptor", Namespace: "default"}}},
			}
			reader, _, _, err := c.GetResource(context.Background(), octx, cv, &v1alpha1.ResourceReference{ElementMeta: v1alpha1.ElementMeta{Name: "data", Version: "1.0.0"}})
			if reader != nil {
				_ = reader.Close()
			}
			if (err != nil) != pushFails {
				t.Fatalf("unexpected fetch result: %v", err)
			}
			if component.closed != 1 {
				t.Errorf("original component closed %d times, want 1", component.closed)
			}
			if len(component.duplicates) != 1 {
				t.Fatalf("got %d resolved handles, want 1", len(component.duplicates))
			}
			if n := component.duplicates[0].closed; n != 1 {
				t.Errorf("resolved component closed %d times, want 1", n)
			}
		})
	}
}

type lifecycleMethod struct {
	ocmcore.AccessMethod
	closed    int
	readerErr error
}

func (m *lifecycleMethod) Reader() (io.ReadCloser, error) {
	if m.readerErr != nil {
		return nil, m.readerErr
	}
	return io.NopCloser(strings.NewReader("payload")), nil
}
func (m *lifecycleMethod) Close() error { m.closed++; return nil }

type lifecycleResource struct {
	ocmcore.ResourceAccess
	method *lifecycleMethod
}

func (r *lifecycleResource) AccessMethod() (ocmcore.AccessMethod, error) { return r.method, nil }

func TestResourceReaderReleasesAccessMethod(t *testing.T) {
	for _, readerFails := range []bool{false, true} {
		name := "success"
		if readerFails {
			name = "reader failure"
		}
		t.Run(name, func(t *testing.T) {
			method := &lifecycleMethod{}
			if readerFails {
				method.readerErr = errors.New("reader failed")
			}
			res := &lifecycleResource{ResourceAccess: &fakeocm.Resource[*ocmcore.ResourceMeta]{Type: "ociBlob"}, method: method}
			reader, _, err := (&Client{}).fetchResourceReader(res, nil)
			if (err != nil) != readerFails {
				t.Fatalf("unexpected reader result: %v", err)
			}
			if reader != nil {
				if method.closed != 0 {
					t.Fatal("access method closed before reader was consumed")
				}
				if _, err := io.ReadAll(reader); err != nil {
					t.Fatal(err)
				}
				if err := reader.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if method.closed != 1 {
				t.Errorf("access method closed %d times, want 1", method.closed)
			}
		})
	}
}
