package controllers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/mandelsoft/vfs/pkg/osfs"
	"github.com/open-component-model/ocm/pkg/contexts/datacontext/attrs/tmpcache"
	"github.com/open-component-model/ocm/pkg/contexts/datacontext/attrs/vfsattr"
	_ "github.com/open-component-model/ocm/pkg/contexts/datacontext/config"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm/pkg/common"
	"github.com/open-component-model/ocm/pkg/contexts/credentials"
	"github.com/open-component-model/ocm/pkg/contexts/datacontext"
	"github.com/open-component-model/ocm/pkg/contexts/ocm"
)

// RoundTripFunc .
type RoundTripFunc func(req *http.Request) *http.Response

// RoundTrip .
func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

// NewTestClient returns *http.Client with Transport replaced to avoid making real calls
func NewTestClient(fn RoundTripFunc) *http.Client {
	return &http.Client{
		Transport: fn,
	}

}

type mockDownloader struct {
	expected []byte
	err      error
}

func (m *mockDownloader) Download(w io.WriterAt) error {
	if _, err := w.WriteAt(m.expected, 0); err != nil {
		return fmt.Errorf("failed to write to mock writer: %w", err)
	}
	return m.err
}

func TestComponentVersionReconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	assert.NoError(t, err)
	err = corev1.AddToScheme(scheme)
	assert.NoError(t, err)
	fakeClient := fake.NewClientBuilder()

	ctx := ocm.New()
	session := ocm.NewSession(nil)
	defer session.Close()
	//expectedBlobContent, err := os.ReadFile(filepath.Join("testdata", "repo.tar.gz"))
	//assert.NoError(t, err)
	//
	//clientFn := func() *http.Client {
	//	return NewTestClient(func(req *http.Request) *http.Response {
	//		return &http.Response{
	//			StatusCode: http.StatusFound,
	//			Status:     http.StatusText(http.StatusFound),
	//			Body:       io.NopCloser(bytes.NewBufferString(`{}`)),
	//			Header: http.Header{
	//				"Location": []string{"https://github.com/Skarlso/test"},
	//			},
	//		}
	//	})
	//}

	//accessSpec := me.New(
	//	"https://github.com/test/test",
	//	"",
	//	"7b1445755ee2527f0bf80ef9eeb59a5d2e6e3e1f",
	//	me.WithClient(clientFn()),
	//	me.WithDownloader(&mockDownloader{
	//		expected: expectedBlobContent,
	//	}),
	//)
	//m, err := accessSpec.AccessMethod(&cpi.DummyComponentVersionAccess{Context: ctx})
	assert.NoError(t, err)
	fs, err := osfs.NewTempFileSystem()
	assert.NoError(t, err)
	vfsattr.Set(ctx, fs)
	tmpcache.Set(ctx, &tmpcache.Attribute{Path: "/tmp"})

	var (
		componentName = "test-name"
		secretName    = "test-secret"
		namespace     = "default"
	)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"creds": []byte("whatever"),
		},
	}
	obj := &v1alpha1.ComponentVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      componentName,
			Namespace: namespace,
		},
		Spec: v1alpha1.ComponentVersionSpec{
			Interval: metav1.Duration{Duration: 10 * time.Minute},
			Name:     "github.com/skarlso/test",
			Version:  "v0.0.1",
			Repository: v1alpha1.Repository{
				URL: "https://github.com/Skarlso/test",
				SecretRef: v1alpha1.SecretRef{
					Name: secretName,
				},
			},
			Verify: v1alpha1.Verify{},
			References: v1alpha1.ReferencesConfig{
				Expand: true,
			},
		},
		Status: v1alpha1.ComponentVersionStatus{},
	}
	client := fakeClient.WithObjects(secret, obj).WithScheme(scheme).Build()
	cvr := ComponentVersionReconciler{
		Scheme: scheme,
		Client: client,
	}

	result, err := cvr.reconcile(context.Background(), ctx, session, obj)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

type mockComponentVersionAccess struct {
	ocm.ComponentVersionAccess
	credContext ocm.Context
}

func (m *mockComponentVersionAccess) GetContext() ocm.Context {
	return m.credContext
}

type mockContext struct {
	ocm.Context
	creds       credentials.Context
	dataContext datacontext.Context
}

func (m *mockContext) CredentialsContext() credentials.Context {
	return m.creds
}

func (m *mockContext) GetAttributes() datacontext.Attributes {
	return m.dataContext.GetAttributes()
}

type mockCredSource struct {
	credentials.Context
	cred credentials.Credentials
	err  error
}

func (m *mockCredSource) GetCredentialsForConsumer(credentials.ConsumerIdentity, ...credentials.IdentityMatcher) (credentials.CredentialsSource, error) {
	return m, m.err
}

func (m *mockCredSource) Credentials(credentials.Context, ...credentials.CredentialsSource) (credentials.Credentials, error) {
	return m.cred, nil
}

type mockCredentials struct {
	value func() string
}

func (m *mockCredentials) Credentials(context credentials.Context, source ...credentials.CredentialsSource) (credentials.Credentials, error) {
	panic("implement me")
}

func (m *mockCredentials) ExistsProperty(name string) bool {
	panic("implement me")
}

func (m *mockCredentials) PropertyNames() sets.String {
	panic("implement me")
}

func (m *mockCredentials) Properties() common.Properties {
	panic("implement me")
}

func (m *mockCredentials) GetProperty(name string) string {
	return m.value()
}
