package fakes

import (
	"bytes"
	"fmt"
	"io"

	"github.com/mandelsoft/logging"
	"github.com/open-component-model/ocm/pkg/contexts/credentials"
	"github.com/open-component-model/ocm/pkg/contexts/datacontext"
	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/attrs/signingattr"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc"
	ocmmetav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/signing"
	ocmruntime "github.com/open-component-model/ocm/pkg/runtime"
	ocmsigning "github.com/open-component-model/ocm/pkg/signing"
	"github.com/open-component-model/ocm/pkg/signing/handlers/rsa"
)

// AccessOptionFunc modifies the resource's access settings.
type AccessOptionFunc func(map[string]any)

// SetAccessType sets a custom access type for a resource.
func SetAccessType(t string) AccessOptionFunc {
	return func(m map[string]any) {
		m["type"] = t
	}
}

// SetAccessRef completely overrides the globalAccess field of the resource.
func SetAccessRef(t string) AccessOptionFunc {
	return func(m map[string]any) {
		m["globalAccess"].(map[string]any)["ref"] = t //nolint:forcetypeassert // fake
	}
}

// Resource presents a simple layout for a resource that AddComponentVersionToRepository will use.
type Resource[M any] struct {
	Name     string
	Labels   ocmmetav1.Labels
	Version  string
	Data     []byte
	Kind     string
	Type     string
	Relation ocmmetav1.ResourceRelation

	// The component that contains this resource. This is a backlink in OCM.
	Component *Component

	// AccessOptions to modify the access of the resource.
	AccessOptions []AccessOptionFunc
}

// Sign defines the two needed values to perform a component signing.
type Sign struct {
	Name    string
	PubKey  []byte
	PrivKey []byte
	Digest  string
}

// Component presents a simple layout for a component. If `Sign` is not empty, it's used to
// sign the component. It should be the byte representation of a private key.
// This has to implement ocm.ComponentVersionAccess.
// Add References. Right now, only resources are supported.
type Component struct {
	ocm.ComponentVersionAccess
	repository *mockRepository
	context    *Context

	Name                string
	Version             string
	Sign                *Sign
	Resources           []*Resource[*compdesc.ResourceMeta]
	ComponentDescriptor *compdesc.ComponentDescriptor
}

// Context defines a mock OCM context.
type Context struct {
	// Make sure our context is compliant with ocm.Context. Implemented methods will be added on a need-to basis.
	ocm.Context
	credentialCtx credentials.Context

	// Components holds all components that are added to the context.
	components map[string][]*Component

	// repo contains all the configured component versions.
	repo *mockRepository

	// attributes contains attributes for this context.
	attributes *mockAttribute
}

func (c *Context) IsAttributesContext() bool {
	return true
}

func (c *Context) AttributesContext() datacontext.AttributesContext {
	return c
}

func (c *Context) AddComponent(component *Component) error {
	// set up the repository context for the component.
	component.repository = c.repo
	component.context = c
	component.ComponentDescriptor = c.constructComponentDescriptor(component)

	// add the component to our global list of components
	c.components[component.Name] = append(c.components[component.Name], component)

	// add the component to the repository for later lookup
	c.repo.cv = append(c.repo.cv, &mockComponentAccess{
		name:    component.Name,
		context: c,
	})

	// add the component to the list of components for this repository
	c.repo.cva = append(c.repo.cva, component)

	if component.Sign != nil {
		resolver := ocm.NewCompoundResolver(c.repo)
		opts := signing.NewOptions(
			signing.Sign(
				ocmsigning.DefaultHandlerRegistry().GetSigner(rsa.Algorithm),
				component.Sign.Name,
			),
			signing.Resolver(resolver),
			signing.PrivateKey(component.Sign.Name, component.Sign.PrivKey),
			signing.Update(),
			signing.VerifyDigests(),
		)

		if err := opts.Complete(signingattr.Get(c)); err != nil {
			return fmt.Errorf("failed to complete signing: %w", err)
		}

		if _, err := signing.Apply(nil, nil, component, opts); err != nil {
			return fmt.Errorf("failed to apply signing: %w", err)
		}
	}

	return nil
}

var _ ocm.Context = &Context{}

// NewFakeOCMContext creates a new fake OCM context.
func NewFakeOCMContext() *Context {
	// create the context
	c := &Context{
		components:    make(map[string][]*Component),
		credentialCtx: credentials.New(),
	}
	attributes := &mockAttribute{context: c}
	c.attributes = attributes

	// create our repository and tie it to the context
	repo := &mockRepository{name: "fake-repo", version: "1.0.0", context: c}

	// add the repository to the context
	c.repo = repo

	return c
}

// Setup context's repository to return. ATM we have a single repository configured that holds all the versions.

func (c *Context) RepositoryForSpec(
	_ ocm.RepositorySpec,
	_ ...credentials.CredentialsSource,
) (ocm.Repository, error) {
	return c.repo, nil
}

func (c *Context) AccessSpecForSpec(spec compdesc.AccessSpec) (ocm.AccessSpec, error) {
	ctx := ocm.New()

	return ctx.AccessSpecForSpec(spec)
}

func (c *Context) OCMContext() ocm.Context { return c }

func (c *Context) GetContext() ocm.Context {
	return c
}

func (c *Context) GetAttributes() datacontext.Attributes {
	return c.attributes
}

func (c *Context) CredentialsContext() credentials.Context {
	return c.credentialCtx
}

func (c *Context) LoggingContext() logging.Context {
	return logging.NewDefault()
}

func (c *Context) Close() error {
	return nil
}

func (c *Context) BlobDigesters() ocm.BlobDigesterRegistry {
	return nil
}

func (c *Context) constructComponentDescriptor(
	component *Component,
) *compdesc.ComponentDescriptor {
	var resources compdesc.Resources

	for _, res := range component.Resources {
		resources = append(resources, compdesc.Resource{
			ResourceMeta: compdesc.ResourceMeta{
				ElementMeta: compdesc.ElementMeta{
					Name:    res.Name,
					Version: res.Version,
					Labels:  res.Labels,
				},
				Type:     res.Type,
				Relation: res.Relation,
			},
			Access: res.AccessSpec(),
		})
	}

	compd := &compdesc.ComponentDescriptor{
		Metadata: compdesc.Metadata{
			ConfiguredVersion: "v2",
		},
		ComponentSpec: compdesc.ComponentSpec{
			ObjectMeta: ocmmetav1.ObjectMeta{
				Name:    component.Name,
				Version: component.Version,
				Provider: ocmmetav1.Provider{
					Name: "acme",
				},
			},
			Resources: resources,
		},
	}

	return compd
}

// ************** Mock Attribute Value **************

type mockAttribute struct {
	context ocm.Context
	datacontext.Attributes
}

func (m *mockAttribute) GetAttribute(name string, def ...any) any { //nolint:revive // fake
	return nil
}

func (m *mockAttribute) GetOrCreateAttribute(name string, creator datacontext.AttributeFactory) any { //nolint:revive // fake
	return creator(m.context)
}

// ************** Mock Repository Value and Functions **************

type mockRepository struct {
	ocm.Repository
	name    string
	version string

	context *Context
	cva     []*Component
	cv      []*mockComponentAccess
}

func (m *mockRepository) LookupComponentVersion(
	name string,
	version string,
) (ocm.ComponentVersionAccess, error) {
	for _, c := range m.cva {
		if c.Name == name && c.Version == version {
			return c, nil
		}
	}

	return nil, fmt.Errorf(
		"failed to find component version in mock repository with name %s and version %s",
		name,
		version,
	)
}

func (m *mockRepository) LookupComponent(name string) (ocm.ComponentAccess, error) {
	for _, ca := range m.cv {
		if ca.name == name {
			return ca, nil
		}
	}

	return nil, fmt.Errorf(
		"component access with name '%s' not configured in mock ocm context",
		name,
	)
}

func (m *mockRepository) Close() error {
	return nil
}

func (m *mockRepository) GetName() string {
	return m.name
}

func (m *mockRepository) GetVersion() string {
	return m.version
}

var _ ocm.Repository = &mockRepository{}

// ************** Mock Component Access Value and Functions **************

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

// ************** Mock Component Version Access Value and Functions **************

var _ ocm.ComponentVersionAccess = &Component{}

func (c *Component) Close() error {
	return nil
}

func (c *Component) Update() error { return nil }

func (c *Component) Dup() (ocm.ComponentVersionAccess, error) {
	return c, nil
}

func (c *Component) Repository() ocm.Repository {
	return c.repository
}

func (c *Component) GetDescriptor() *compdesc.ComponentDescriptor {
	return c.ComponentDescriptor
}

func (c *Component) GetContext() ocm.Context {
	return c.context
}

func (c *Component) GetResources() []ocm.ResourceAccess {
	accesses := make([]ocm.ResourceAccess, 0, len(c.Resources))

	for _, r := range c.Resources {
		accesses = append(accesses, r)
	}

	return accesses
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

func (c *Component) GetName() string {
	return c.Name
}

func (c *Component) GetVersion() string {
	return c.Version
}

// ************** Mock Resource Access Value and Functions **************

var _ ocm.ResourceAccess = &Resource[*ocm.ResourceMeta]{}

func (r *Resource[M]) Meta() *ocm.ResourceMeta {
	return &ocm.ResourceMeta{
		ElementMeta: compdesc.ElementMeta{
			Name:    r.Name,
			Version: r.Version,
			Labels:  r.Labels,
		},
		Type:     r.Type,
		Relation: r.Relation,
	}
}

func (r *Resource[M]) GetOCMContext() ocm.Context {
	return r.Component.context
}

func (r *Resource[M]) ReferenceHint() string {
	return "I'm a fake provider"
}

func (r *Resource[M]) GlobalAccess() ocm.AccessSpec {
	spec, err := r.Access()
	if err != nil {
		panic(err)
	}

	return spec
}

func (r *Resource[M]) BlobAccess() (ocm.BlobAccess, error) {
	return nil, nil
}

func (r *Resource[M]) GetComponentVersion() (ocm.ComponentVersionAccess, error) {
	return nil, nil
}

func (r *Resource[M]) ComponentVersion() ocm.ComponentVersionAccess {
	return r.Component
}

// Access provides some canned settings. This will later on by made configurable as is in getComponentMock.
func (r *Resource[M]) Access() (ocm.AccessSpec, error) {
	const size = 29047129
	accObj := map[string]any{
		"globalAccess": map[string]any{
			"digest":    "sha256:7f0168496f273c1e2095703a050128114d339c580b0906cd124a93b66ae471e2",
			"mediaType": "application/vnd.docker.distribution.manifest.v2+tar+gzip",
			"ref":       "ghcr.io/mandelsoft/cnudie/component-descriptors/github.com/vasu1124/introspect",
			"size":      size,
			"type":      "ociBlob",
		},
		"localReference": "sha256:7f0168496f273c1e2095703a050128114d339c580b0906cd124a93b66ae471e2",
		"mediaType":      "application/vnd.docker.distribution.manifest.v2+tar+gzip",
		"type":           "localBlob",
	}

	for _, opt := range r.AccessOptions {
		opt(accObj)
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
func (r *Resource[M]) AccessMethod() (ocm.AccessMethod, error) {
	return r, nil
}

// ************** Mock Access Method **************

var _ ocm.AccessMethod = &Resource[*ocm.ResourceMeta]{}

func (r *Resource[M]) Get() ([]byte, error) {
	return r.Data, nil
}

func (r *Resource[M]) Reader() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewBuffer(r.Data)), nil
}

func (r *Resource[M]) Close() error {
	return nil
}

func (r *Resource[M]) GetKind() string {
	return r.Kind
}

func (r *Resource[M]) AccessSpec() ocm.AccessSpec {
	// What?
	acc, _ := r.Access()

	return acc
}

func (r *Resource[M]) MimeType() string {
	return r.Type
}

func (r *Resource[M]) Dup() (ocm.AccessMethod, error) {
	return nil, nil
}

func (r *Resource[M]) IsLocal() bool {
	return false
}

func (r *Resource[M]) AsBlobAccess() ocm.BlobAccess {
	return nil
}
