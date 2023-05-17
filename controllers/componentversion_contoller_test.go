// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/open-component-model/ocm/pkg/common/accessobj"
	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	ocmdesc "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc"
	ocmmetav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	v1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/repositories/comparch"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/repositories/genericocireg"

	"github.com/open-component-model/ocm-controller/pkg/ocm/fakes"
)

func TestComponentVersionReconcile(t *testing.T) {
	var secretName = "test-secret"
	cv := DefaultComponent.DeepCopy()
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: cv.Namespace,
		},
		Data: map[string][]byte{
			"creds": []byte("whatever"),
		},
	}
	client := env.FakeKubeClient(WithObjects(secret, cv))
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
	fakeOcm.VerifyComponentReturns(true, nil)
	fakeOcm.GetLatestComponentVersionReturns("v0.0.1", nil)
	recorder := &record.FakeRecorder{
		Events:        make(chan string, 32),
		IncludeObject: true,
	}

	cvr := ComponentVersionReconciler{
		Scheme:        env.scheme,
		Client:        client,
		EventRecorder: recorder,
		OCMClient:     fakeOcm,
	}
	_, err := cvr.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      cv.Name,
			Namespace: cv.Namespace,
		},
	})
	require.NoError(t, err)

	t.Log("verifying updated object status")
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      cv.Name,
		Namespace: cv.Namespace,
	}, cv)
	require.NoError(t, err)

	assert.Len(t, cv.Status.ComponentDescriptor.References, 1)
	assert.Equal(t, "test-ref-1", cv.Status.ComponentDescriptor.References[0].Name)
	assert.True(t, conditions.IsTrue(cv, meta.ReadyCondition))

	close(recorder.Events)
	event := ""
	for e := range recorder.Events {
		if strings.Contains(e, "Reconciliation finished, next run in") {
			event = e
			break
		}
	}
	assert.Contains(t, event, "Reconciliation finished, next run in")
	assert.Contains(t, event, "kind=ComponentVersion")
}

func TestComponentVersionReconcileFailure(t *testing.T) {
	cv := DefaultComponent.DeepCopy()
	cv.Status.ReconciledVersion = "invalid"
	client := env.FakeKubeClient(WithObjects(cv))
	recorder := &record.FakeRecorder{
		Events:        make(chan string, 32),
		IncludeObject: true,
	}

	fakeOcm := &fakes.MockFetcher{}
	cvr := ComponentVersionReconciler{
		Scheme:        env.scheme,
		Client:        client,
		EventRecorder: recorder,
		OCMClient:     fakeOcm,
	}
	_, err := cvr.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      cv.Name,
			Namespace: cv.Namespace,
		},
	})
	assert.EqualError(t, err, "failed to check version: failed to parse latest version: Invalid Semantic Version")

	t.Log("verifying updated object status")
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      cv.Name,
		Namespace: cv.Namespace,
	}, cv)
	require.NoError(t, err)

	assert.True(t, conditions.IsFalse(cv, meta.ReadyCondition))
	assert.True(t, conditions.IsTrue(cv, meta.StalledCondition))

	close(recorder.Events)
	found, event := false, ""
	for e := range recorder.Events {
		if strings.Contains(e, "failed to check version") {
			found, event = true, e
			break
		}
	}
	assert.True(t, found)
	assert.Contains(t, event, "failed to check version: failed to parse latest version: Invalid Semantic Version")
	assert.Contains(t, event, "kind=ComponentVersion")
}

func TestComponentVersionSemverCheck(t *testing.T) {
	semverTests := []struct {
		description       string
		givenVersion      string
		latestVersion     string
		reconciledVersion string
		expectedUpdate    bool
		expectedErr       string
		allowRollback     bool
	}{
		{
			description:       "current reconciled version is latest and satisfies given semver constraint",
			givenVersion:      ">=0.0.2",
			reconciledVersion: "0.0.3",
			expectedUpdate:    false,
			latestVersion:     "0.0.3",
		},
		{
			description:       "given version requires component update",
			givenVersion:      ">=0.0.2",
			reconciledVersion: "0.0.1",
			latestVersion:     "0.0.2",
			expectedUpdate:    true,
		},
		{
			description:       "equaling a version should return that specific version and will trigger an update",
			givenVersion:      "=0.0.3",
			reconciledVersion: "0.0.2",
			latestVersion:     "0.0.3",
			expectedUpdate:    true,
		},
		{
			description:       "missing latest version should not force a downgrade",
			givenVersion:      "<=0.0.3",
			reconciledVersion: "0.0.3",
			latestVersion:     "0.0.2",
			expectedUpdate:    false,
		},
		{
			description:       "use older version is rollback is enabled",
			givenVersion:      "<=0.0.3",
			reconciledVersion: "0.0.3",
			latestVersion:     "0.0.2",
			allowRollback:     true,
			expectedUpdate:    true,
		},
		{
			description:       "don't allow rollback if it isn't enabled",
			givenVersion:      "0.0.3",
			reconciledVersion: "0.0.4",
			latestVersion:     "0.0.3",
			expectedUpdate:    false,
		},
		{
			description:       "make sure update doesn't occur if allowRollback is disabled but versions match",
			givenVersion:      ">=0.0.3",
			reconciledVersion: "0.0.4",
			latestVersion:     "0.0.4",
			expectedUpdate:    false,
		},
	}
	for i, tt := range semverTests {
		t.Run(fmt.Sprintf("%d: %s", i, tt.description), func(t *testing.T) {
			obj := DefaultComponent.DeepCopy()
			obj.Spec.Version.Semver = tt.givenVersion
			obj.Spec.Version.AllowRollback = tt.allowRollback
			obj.Status.ReconciledVersion = tt.reconciledVersion
			fakeClient := env.FakeKubeClient(WithObjects(obj))
			fakeOcm := &fakes.MockFetcher{}
			fakeOcm.GetLatestComponentVersionReturns(tt.latestVersion, nil)
			recorder := &record.FakeRecorder{
				Events: make(chan string, 32),
			}

			cvr := ComponentVersionReconciler{
				Scheme:        env.scheme,
				Client:        fakeClient,
				EventRecorder: recorder,
				OCMClient:     fakeOcm,
			}
			update, _, err := cvr.checkVersion(context.Background(), nil, obj)
			require.NoError(t, err)
			require.Equal(t, tt.expectedUpdate, update)

			close(recorder.Events)
			for e := range recorder.Events {
				switch {
				case tt.expectedUpdate:
					assert.Contains(t, e, "Version check succeeded, found latest")
				}
			}
		})
	}
}

type mockComponent struct {
	ocm.ComponentVersionAccess
	descriptor *ocmdesc.ComponentDescriptor
	t          *testing.T
}

func (m *mockComponent) GetName() string {
	return m.descriptor.ComponentSpec.Name
}

func (m *mockComponent) GetDescriptor() *ocmdesc.ComponentDescriptor {
	return m.descriptor
}

// how to get resource access for resource?
func (m *mockComponent) GetResource(id ocmmetav1.Identity) (ocm.ResourceAccess, error) {
	r, err := m.descriptor.GetResourceByIdentity(id)
	if err != nil {
		return nil, err
	}
	return &mockResource{resource: r, ctx: ocm.DefaultContext()}, nil
}

func (m *mockComponent) Repository() ocm.Repository {
	return &genericocireg.Repository{}
}

func (m *mockComponent) Dup() (ocm.ComponentVersionAccess, error) {
	return m, nil
}

func (m *mockComponent) Close() error {
	return nil
}

type mockResource struct {
	ctx      ocm.Context
	resource ocmdesc.Resource
}

func (r *mockResource) Access() (ocm.AccessSpec, error) {
	return r.ctx.AccessSpecForSpec(r.resource.Access)
}

func (r *mockResource) AccessMethod() (ocm.AccessMethod, error) {
	ca, err := comparch.New(r.ctx, accessobj.ACC_CREATE, nil, nil, nil, 0600)
	if err != nil {
		return nil, err
	}
	spec, err := r.ctx.AccessSpecForSpec(r.resource.Access)
	if err != nil {
		return nil, err
	}
	return spec.AccessMethod(ca)
}

func (r *mockResource) Meta() *ocm.ResourceMeta {
	return &ocm.ResourceMeta{ElementMeta: *r.resource.GetMeta()}
}
