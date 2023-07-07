package fakes

import (
	"fmt"

	"github.com/open-component-model/ocm/pkg/contexts/credentials"
	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	ocmmetav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
)

// Resource presents a simple layout for a resource that AddComponentVersionToRepository will use.
type Resource struct {
	Name    string
	Version string
	Data    string
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
	Resources []Resource
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
	// add the component to our global list of components
	c.components[component.Name] = append(c.components[component.Name], component)

	// add the component to the repository for later lookup
	c.repo.cv = append(c.repo.cv, &mockComponentAccess{
		name:    component.Name,
		context: c,
	})
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
	cva     *Component
	cv      []*mockComponentAccess
}

func (m *mockRepository) LookupComponentVersion(name string, version string) (ocm.ComponentVersionAccess, error) {
	return m.cva, nil
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
	return nil, nil
}
