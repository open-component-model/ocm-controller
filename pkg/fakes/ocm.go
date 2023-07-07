package fakes

import (
	"bytes"
	"fmt"
	"io"

	"github.com/open-component-model/ocm/pkg/contexts/credentials"
	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc"
	ocmmetav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	ocmruntime "github.com/open-component-model/ocm/pkg/runtime"
)

// Resource presents a simple layout for a resource that AddComponentVersionToRepository will use.
type Resource struct {
	Name    string
	Version string
	Data    []byte
	Kind    string
	Type    string

	// The component that contains this resource. This is a backlink in OCM.
	Component *Component
}

// Sign defines the two needed values to perform a component signing.
type Sign struct {
	Name string
	Key  []byte
}

// Component presents a simple layout for a component. If `Sign` is not empty, it's used to
// sign the component. It should be the byte representation of a private key.
// This has to implement ocm.ComponentVersionAccess.
type Component struct {
	ocm.ComponentVersionAccess
	repository *mockRepository

	Name      string
	Version   string
	Sign      *Sign
	Resources []*Resource
}

// Context defines a mock OCM context.
type Context struct {
	// Make sure our context is compliant with ocm.Context. Implemented methods will be added on a need-to basis.
	ocm.Context

	// Components holds all components that are added to the context.
	components map[string][]*Component

	// repo contains all the configured component versions.
	repo *mockRepository
}

func (c *Context) AddComponent(component *Component) {
	// set up the repository context for the component.
	component.repository = c.repo

	// add the component to our global list of components
	c.components[component.Name] = append(c.components[component.Name], component)

	// add the component to the repository for later lookup
	c.repo.cv = append(c.repo.cv, &mockComponentAccess{
		name:    component.Name,
		context: c,
	})

	// add the component to the list of components for this repository
	c.repo.cva = append(c.repo.cva, component)
}

var _ ocm.Context = &Context{}

// NewFakeOCMContext creates a new fake OCM context.
func NewFakeOCMContext() *Context {
	// create the context
	c := &Context{
		components: make(map[string][]*Component),
	}

	// create our repository and tie it to the context
	repo := &mockRepository{context: c}

	// add the repository to the context
	c.repo = repo

	return c
}

// Setup context's repository to return. ATM we have a single repository configured that holds all the versions.

func (c *Context) RepositoryForSpec(spec ocm.RepositorySpec, creds ...credentials.CredentialsSource) (ocm.Repository, error) {
	return c.repo, nil
}

func (c *Context) Close() error {
	return nil
}

// ************** Mock Repository Values and Functions **************

type mockRepository struct {
	ocm.Repository

	context *Context
	cva     []*Component
	cv      []*mockComponentAccess
}

func (m *mockRepository) LookupComponentVersion(name string, version string) (ocm.ComponentVersionAccess, error) {
	for _, c := range m.cva {
		if c.Name == name && c.Version == version {
			return c, nil
		}
	}

	return nil, fmt.Errorf("failed to find component version in mock repository with name %s and version %s", name, version)
}

func (m *mockRepository) LookupComponent(name string) (ocm.ComponentAccess, error) {
	for _, ca := range m.cv {
		if ca.name == name {
			return ca, nil
		}
	}

	return nil, fmt.Errorf("component access with name '%s' not configured in mock ocm context", name)
}

func (m *mockRepository) Close() error {
	return nil
}

var _ ocm.Repository = &mockRepository{}

// ************** Mock Component Access Values and Functions **************

type mockComponentAccess struct {
	ocm.ComponentAccess
	context *Context

	name string
}

func (m *mockComponentAccess) Close() error {
	return nil
}

func (m *mockComponentAccess) ListVersions() ([]string, error) {
	var versions []string

	for _, v := range m.context.components[m.name] {
		versions = append(versions, v.Version)
	}
	return versions, nil
}

var _ ocm.ComponentAccess = &mockComponentAccess{}

// ************** Mock Component Version Access Values and Functions **************

var _ ocm.ComponentVersionAccess = &Component{}

func (c *Component) Close() error {
	return nil
}

func (c *Component) Dup() (ocm.ComponentVersionAccess, error) {
	return c, nil
}

func (c *Component) Repository() ocm.Repository {
	return c.repository
}

func (c *Component) GetResource(meta ocmmetav1.Identity) (ocm.ResourceAccess, error) {
	for _, r := range c.Resources {
		//  && r.Version == meta["version"] --> add this at some point
		if r.Name == meta["name"] {
			return r, nil
		}
	}

	return nil, fmt.Errorf("failed to find resource on component with identity: %v", meta)
}

// ************** Mock Resource Access Values and Functions **************

var _ ocm.ResourceAccess = &Resource{}

func (r *Resource) Meta() *ocm.ResourceMeta {
	return &ocm.ResourceMeta{ElementMeta: compdesc.ElementMeta{
		Name:    r.Name,
		Version: r.Version,
	}}
}

func (r *Resource) ComponentVersion() ocm.ComponentVersionAccess {
	return r.Component
}

// Access provides some canned settings. This will later on by made configurable as is in getComponentMock.
func (r *Resource) Access() (ocm.AccessSpec, error) {
	accObj := map[string]any{
		"globalAccess": map[string]any{
			"digest":    "sha256:7f0168496f273c1e2095703a050128114d339c580b0906cd124a93b66ae471e2",
			"mediaType": "application/vnd.docker.distribution.manifest.v2+tar+gzip",
			"ref":       "ghcr.io/mandelsoft/cnudie/component-descriptors/github.com/vasu1124/introspect",
			"size":      29047129,
			"type":      "ociBlob",
		},
		"localReference": "sha256:7f0168496f273c1e2095703a050128114d339c580b0906cd124a93b66ae471e2",
		"mediaType":      "application/vnd.docker.distribution.manifest.v2+tar+gzip",
		"type":           "localBlob",
	}

	acc, err := ocmruntime.ToUnstructuredVersionedTypedObject(
		ocmruntime.UnstructuredTypedObject{
			Object: accObj,
		},
	)
	if err != nil {
		return nil, err
	}
	ctx := ocm.New()
	return ctx.AccessSpecForSpec(acc)
}

// AccessMethod implementation for this is provided by the resource itself to avoid even further indirections.
func (r *Resource) AccessMethod() (ocm.AccessMethod, error) {
	return r, nil
}

// ************** Mock Access Method **************

var _ ocm.AccessMethod = &Resource{}

func (r *Resource) Get() ([]byte, error) {
	return r.Data, nil
}

func (r *Resource) Reader() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewBuffer(r.Data)), nil
}

func (r *Resource) Close() error {
	return nil
}

func (r *Resource) GetKind() string {
	return r.Kind
}

func (r *Resource) AccessSpec() ocm.AccessSpec {
	// What?
	acc, _ := r.Access()

	return acc
}

func (r *Resource) MimeType() string {
	return r.Type
}
