// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"fmt"
	"testing"
	"time"

	ococm "github.com/open-component-model/ocm-controller/pkg/ocm"
	_ "github.com/open-component-model/ocm/pkg/contexts/datacontext/config"
	v1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	ocmdesc "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
)

func TestComponentVersionReconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	assert.NoError(t, err)
	err = corev1.AddToScheme(scheme)
	assert.NoError(t, err)
	fakeClient := fake.NewClientBuilder()

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
			Interval:  metav1.Duration{Duration: 10 * time.Minute},
			Component: "github.com/skarlso/root",
			Version: v1alpha1.Version{
				Semver: "v0.0.1",
			},
			Repository: v1alpha1.Repository{
				URL: "https://github.com/Skarlso/test",
				SecretRef: v1alpha1.SecretRef{
					Name: secretName,
				},
			},
			Verify: []v1alpha1.Signature{},
			References: v1alpha1.ReferencesConfig{
				Expand: true,
			},
		},
		Status: v1alpha1.ComponentVersionStatus{},
	}
	client := fakeClient.WithObjects(secret, obj).WithScheme(scheme).Build()
	root := &mockComponent{
		t: t,
		descriptor: &ocmdesc.ComponentDescriptor{
			ComponentSpec: ocmdesc.ComponentSpec{
				ObjectMeta: v1.ObjectMeta{
					Name:    "github.com/skarlso/root",
					Version: "v0.0.1",
				},
				References: ocmdesc.References{
					{
						ElementMeta: ocmdesc.ElementMeta{
							Name:    "test-ref-1",
							Version: "v0.0.1",
						},
						ComponentName: "github.com/skarlso/embedded",
					},
				},
			},
		},
	}
	embedded := &mockComponent{
		descriptor: &ocmdesc.ComponentDescriptor{
			ComponentSpec: ocmdesc.ComponentSpec{
				ObjectMeta: v1.ObjectMeta{
					Name:    "github.com/skarlso/embedded",
					Version: "v0.0.1",
				},
			},
		},
	}
	cvr := ComponentVersionReconciler{
		Scheme: scheme,
		Client: client,
		OCMClient: &mockFetcher{
			verified: true,
			t:        t,
			cv: map[string]ocm.ComponentVersionAccess{
				"github.com/skarlso/embedded": embedded,
				"github.com/skarlso/root":     root,
			},
		},
	}
	_, err = cvr.reconcile(context.Background(), obj, "0.1.0")
	assert.NoError(t, err)
	assert.Len(t, obj.Status.ComponentDescriptor.References, 1)
	assert.Equal(t, "test-ref-1", obj.Status.ComponentDescriptor.References[0].Name)
}

func TestComponentVersionSemverCheck(t *testing.T) {
	semverTests := []struct {
		description       string
		givenVersion      string
		latestVersion     string
		reconciledVersion string
		expectedUpdate    bool
		expectedErr       string
	}{
		{
			description:       "current reconciled version satisfies given semver constraint",
			givenVersion:      ">=0.0.2",
			reconciledVersion: "0.0.3",
			expectedUpdate:    false,
			latestVersion:     "0.0.1",
		},
		{
			description:       "given version requires component update",
			givenVersion:      ">=0.0.2",
			reconciledVersion: "0.0.1",
			latestVersion:     "0.0.2",
			expectedUpdate:    true,
		},
		{
			description:       "latest available version does not satisfy given semver constraint",
			givenVersion:      "=0.0.2",
			reconciledVersion: "0.0.1",
			latestVersion:     "0.0.1",
			expectedUpdate:    false,
		},
	}
	for i, tt := range semverTests {
		t.Run(fmt.Sprintf("%d: %s", i, tt.description), func(t *testing.T) {
			require := require.New(t)
			scheme := runtime.NewScheme()
			err := v1alpha1.AddToScheme(scheme)
			require.NoError(err)
			err = corev1.AddToScheme(scheme)
			require.NoError(err)

			obj := &v1alpha1.ComponentVersion{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-name",
					Namespace: "default",
				},
				Spec: v1alpha1.ComponentVersionSpec{
					Component: "github.com/skarlso/root",
					Version: v1alpha1.Version{
						Semver: tt.givenVersion,
					},
				},
				Status: v1alpha1.ComponentVersionStatus{
					ReconciledVersion: tt.reconciledVersion,
				},
			}

			fakeClient := fake.NewClientBuilder()
			client := fakeClient.WithObjects(&corev1.Secret{}, obj).WithScheme(scheme).Build()

			cvr := ComponentVersionReconciler{
				Scheme: scheme,
				Client: client,
				OCMClient: &mockFetcher{
					t:             t,
					latestVersion: tt.latestVersion,
				},
			}
			update, _, err := cvr.checkVersion(context.Background(), obj)
			require.NoError(err)
			require.Equal(tt.expectedUpdate, update)
		})
	}
}

type mockFetcher struct {
	getComponentErr error
	verifyErr       error
	getVersionErr   error
	cv              map[string]ocm.ComponentVersionAccess
	t               *testing.T
	verified        bool
	latestVersion   string
}

func (m *mockFetcher) GetComponentVersion(ctx context.Context, obj *v1alpha1.ComponentVersion, name, version string) (ocm.ComponentVersionAccess, error) {
	m.t.Logf("called GetComponentVersion with name %s and version %s", name, version)
	return m.cv[name], m.getComponentErr
}

func (m *mockFetcher) VerifyComponent(ctx context.Context, obj *v1alpha1.ComponentVersion) (bool, error) {
	return m.verified, m.verifyErr
}

func (m *mockFetcher) GetLatestComponentVersion(ctx context.Context, obj *v1alpha1.ComponentVersion) (string, error) {
	return m.latestVersion, m.getVersionErr
}

func (m *mockFetcher) ListComponentVersions(ocmCtx ocm.Context, session ocm.Session, obj *v1alpha1.ComponentVersion) ([]ococm.Version, error) {
	return []ococm.Version{}, m.getVersionErr
}

type mockComponent struct {
	descriptor *ocmdesc.ComponentDescriptor
	ocm.ComponentVersionAccess
	t *testing.T
}

func (m *mockComponent) GetDescriptor() *ocmdesc.ComponentDescriptor {
	return m.descriptor
}
