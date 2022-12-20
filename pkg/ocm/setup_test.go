// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package ocm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/distribution/distribution/v3/configuration"
	dcontext "github.com/distribution/distribution/v3/context"
	"github.com/distribution/distribution/v3/registry/handlers"
	_ "github.com/distribution/distribution/v3/registry/storage/driver/filesystem"
	"github.com/phayes/freeport"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/open-component-model/ocm/pkg/common/accessio"
	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/attrs/signingattr"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc"
	ocmmetav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	ocmreg "github.com/open-component-model/ocm/pkg/contexts/ocm/repositories/ocireg"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/resourcetypes"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/signing"
	"github.com/open-component-model/ocm/pkg/mime"
	ocmsigning "github.com/open-component-model/ocm/pkg/signing"
	"github.com/open-component-model/ocm/pkg/signing/handlers/rsa"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
)

const (
	Signature = "test-signature"
	SignAlgo  = rsa.Algorithm
)

type testEnv struct {
	testServer    *httptest.Server
	repositoryURL string
	scheme        *runtime.Scheme
	obj           []client.Object
}

// FakeKubeClientOption defines options to construct a fake kube client. There are some defaults involved.
// Scheme gets corev1 and v1alpha1 schemes by default. Anything that is passed in will override current
// defaults.
type FakeKubeClientOption func(testEnv *testEnv)

// WithScheme provides an option to set the scheme.
func WithScheme(scheme *runtime.Scheme) FakeKubeClientOption {
	return func(testEnv *testEnv) {
		testEnv.scheme = scheme
	}
}

// WithObjects provides an option to set objects for the fake client.
func WithObjets(obj ...client.Object) FakeKubeClientOption {
	return func(testEnv *testEnv) {
		testEnv.obj = obj
	}
}

// FakeKubeClient creates a fake kube client with some defaults and optional arguments.
func (t *testEnv) FakeKubeClient(opts ...FakeKubeClientOption) client.Client {
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	t.scheme = scheme

	for _, o := range opts {
		o(t)
	}
	return fake.NewClientBuilder().WithScheme(t.scheme).WithObjects(t.obj...).Build()
}

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
type Component struct {
	Name    string
	Version string
	Sign    *Sign
}

func (t *testEnv) AddComponentVersionToRepository(component Component, resources ...Resource) error {
	octx := ocm.ForContext(context.Background())
	target, err := octx.RepositoryForSpec(ocmreg.NewRepositorySpec(t.repositoryURL, nil))

	if err != nil {
		return fmt.Errorf("failed to create repository for spec: %w", err)
	}
	defer target.Close()

	comp, err := target.LookupComponent(component.Name)
	if err != nil {
		return fmt.Errorf("failed to look up component: %w", err)
	}

	compvers, err := comp.NewVersion(component.Version, true)
	if err != nil {
		return fmt.Errorf("failed to create new version '%s': %w", component.Version, err)
	}
	defer compvers.Close()

	for _, resource := range resources {
		err = compvers.SetResourceBlob(
			&compdesc.ResourceMeta{
				ElementMeta: compdesc.ElementMeta{
					Name:    resource.Name,
					Version: resource.Version,
				},
				Type:     resourcetypes.BLOB,
				Relation: ocmmetav1.LocalRelation,
			},
			accessio.BlobAccessForString(mime.MIME_TEXT, resource.Data),
			"", nil,
		)
		if err != nil {
			return fmt.Errorf("failed to set resource blob: %w", err)
		}
	}

	if err := comp.AddVersion(compvers); err != nil {
		return fmt.Errorf("failed to add version: %w", err)
	}

	if component.Sign != nil {
		resolver := ocm.NewCompoundResolver(target)
		opts := signing.NewOptions(
			signing.Sign(ocmsigning.DefaultHandlerRegistry().GetSigner(SignAlgo), component.Sign.Name),
			signing.Resolver(resolver),
			signing.PrivateKey(component.Sign.Name, component.Sign.Key),
			signing.Update(), signing.VerifyDigests(),
		)
		if err := opts.Complete(signingattr.Get(octx)); err != nil {
			return fmt.Errorf("failed to complete signing: %w", err)
		}

		if _, err := signing.Apply(nil, nil, compvers, opts); err != nil {
			return fmt.Errorf("failed to apply signing: %w", err)
		}
	}

	return nil
}

var env = testEnv{}

// Server is a registry server
// It wraps the http.Server
type Server struct {
	http.Server
	logger dcontext.Logger
	config *configuration.Configuration
}

// New creates a new oci registry server
func New(ctx context.Context, addr string) (*Server, error) {
	config, err := getConfig(addr)
	if err != nil {
		return nil, fmt.Errorf("could not get config: %w", err)
	}
	app := handlers.NewApp(ctx, config)
	logger := dcontext.GetLogger(app)
	return &Server{
		http.Server{
			Addr:              addr,
			Handler:           app,
			ReadHeaderTimeout: 1 * time.Second,
		},
		logger,
		config,
	}, nil
}

func getConfig(addr string) (*configuration.Configuration, error) {
	config := &configuration.Configuration{}
	config.HTTP.Addr = addr
	config.HTTP.DrainTimeout = time.Duration(10) * time.Second
	config.Storage = map[string]configuration.Parameters{"inmemory": map[string]interface{}{}}
	config.HTTP.DrainTimeout = time.Duration(10) * time.Second
	return config, nil
}

func TestMain(m *testing.M) {
	ctx := context.Background()
	port, err := freeport.GetFreePort()
	if err != nil {
		panic(fmt.Errorf("could not get free port: %w", err))
	}
	app, err := New(ctx, fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		panic(fmt.Errorf("could not create registry server: %w", err))
	}

	env.testServer = httptest.NewServer(app.Handler)
	env.repositoryURL = env.testServer.URL
	defer env.testServer.Close()

	exitCode := m.Run()
	os.Exit(exitCode)
}
