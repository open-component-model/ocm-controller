// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"fmt"
	"testing"

	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	ocmdesc "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc"
	v1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/open-component-model/ocm-controller/pkg/ocm/fakes"
)

func TestComponentVersionReconcile(t *testing.T) {
	var secretName = "test-secret"
	cv, err := env.CreateComponentVersion(WithComponentVersionPatch([]byte(`spec:
  repository:
    secretRef:
      name: test-overwrite
`)))
	require.NoError(t, err)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: cv.Namespace,
		},
		Data: map[string][]byte{
			"creds": []byte("whatever"),
		},
	}
	client := env.FakeKubeClient(WithObjets(secret, cv))
	root := &mockComponent{
		t: t,
		descriptor: &ocmdesc.ComponentDescriptor{
			ComponentSpec: ocmdesc.ComponentSpec{
				ObjectMeta: v1.ObjectMeta{
					Name:    cv.Spec.Component,
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
	fakeOcm := &fakes.MockFetcher{}
	fakeOcm.VerifyComponentReturns(true, nil)
	fakeOcm.GetComponentVersionReturnsForName(embedded.descriptor.ComponentSpec.Name, embedded, nil)
	fakeOcm.GetComponentVersionReturnsForName(root.descriptor.ComponentSpec.Name, root, nil)
	cvr := ComponentVersionReconciler{
		Scheme:    env.scheme,
		Client:    client,
		OCMClient: fakeOcm,
	}
	_, err = cvr.reconcile(context.Background(), cv, "0.0.1")
	assert.NoError(t, err)
	assert.Len(t, cv.Status.ComponentDescriptor.References, 1)
	assert.Equal(t, "test-ref-1", cv.Status.ComponentDescriptor.References[0].Name)
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
			obj, err := env.CreateComponentVersion(WithComponentVersionPatch([]byte(fmt.Sprintf(`spec:
  version:
    semver: %q
status:
  reconciledVersion: %q`, tt.givenVersion, tt.reconciledVersion))))
			require.NoError(t, err)
			fakeClient := env.FakeKubeClient(WithObjets(obj))
			fakeOcm := &fakes.MockFetcher{}
			fakeOcm.GetLatestComponentVersionReturns(tt.latestVersion, nil)
			cvr := ComponentVersionReconciler{
				Scheme:    env.scheme,
				Client:    fakeClient,
				OCMClient: fakeOcm,
			}
			update, _, err := cvr.checkVersion(context.Background(), obj)
			require.NoError(t, err)
			require.Equal(t, tt.expectedUpdate, update)
		})
	}
}

type mockComponent struct {
	descriptor *ocmdesc.ComponentDescriptor
	ocm.ComponentVersionAccess
	t *testing.T
}

func (m *mockComponent) GetDescriptor() *ocmdesc.ComponentDescriptor {
	return m.descriptor
}

func (m *mockComponent) Close() error {
	return nil
}
