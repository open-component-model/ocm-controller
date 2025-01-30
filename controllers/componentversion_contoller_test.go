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

	ocmdesc "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc"
	v1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	ocmfake "github.com/open-component-model/ocm-controller/pkg/fakes"
	"github.com/open-component-model/ocm-controller/pkg/ocm/fakes"
)

func TestComponentVersionReconcile(t *testing.T) {
	secretName := "test-secret"
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
	root := &ocmfake.Component{
		Name:    cv.Spec.Component,
		Version: "v0.0.1",
		ComponentDescriptor: &ocmdesc.ComponentDescriptor{
			ComponentSpec: ocmdesc.ComponentSpec{
				ObjectMeta: v1.ObjectMeta{
					Name:    cv.Spec.Component,
					Version: "v0.0.1",
					Labels: []v1.Label{
						{
							Name:    "ocm.software/label-value",
							Value:   []byte("\"string-value\""),
							Version: "1.0.0",
						},
						{
							Name:    "ocm.software/label-bool",
							Value:   []byte("true"),
							Version: "1.0.0",
						},
					},
				},
				References: ocmdesc.References{
					{
						ElementMeta: ocmdesc.ElementMeta{
							Name:    "test-ref-1",
							Version: "v0.0.1",
						},
						ComponentName: "github.com/open-component-model/embedded",
					},
				},
			},
		},
	}

	embedded := &ocmfake.Component{
		ComponentDescriptor: &ocmdesc.ComponentDescriptor{
			ComponentSpec: ocmdesc.ComponentSpec{
				ObjectMeta: v1.ObjectMeta{
					Name:    "github.com/open-component-model/embedded",
					Version: "v0.0.1",
					Labels: []v1.Label{
						{
							Name:    "ocm.software/label-value-2",
							Value:   []byte("1.32"),
							Version: "1.0.1",
						},
					},
				},
			},
		},
	}

	fakeOcm := &fakes.MockFetcher{}
	fakeOcm.VerifyComponentReturns(true, nil)
	fakeOcm.GetComponentVersionReturnsForName(embedded.ComponentDescriptor.ComponentSpec.Name, embedded, nil)
	fakeOcm.GetComponentVersionReturnsForName(root.ComponentDescriptor.ComponentSpec.Name, root, nil)
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

	t.Log("checking label values")
	nns := types.NamespacedName{Name: cv.Status.ComponentDescriptor.ComponentDescriptorRef.Name, Namespace: cv.Status.ComponentDescriptor.ComponentDescriptorRef.Namespace}
	cd := &v1alpha1.ComponentDescriptor{}
	err = client.Get(context.Background(), nns, cd)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"ocm.software/label-value": "string-value",
		"ocm.software/label-bool":  "true",
	}, cd.Labels)

	nns = types.NamespacedName{Name: cv.Status.ComponentDescriptor.References[0].ComponentDescriptorRef.Name, Namespace: cv.Namespace}
	err = client.Get(context.Background(), nns, cd)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"ocm.software/label-value-2": "1.32",
	}, cd.Labels)

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

func TestComponentVersionWithTransferReconcile(t *testing.T) {
	secretName := "test-secret"
	cv := DefaultComponent.DeepCopy()
	cv.Spec.Destination = &v1alpha1.Repository{
		URL: "github.com/open-component-model/internal-test",
	}
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
	root := &ocmfake.Component{
		Name:    cv.Spec.Component,
		Version: "v0.0.1",
		ComponentDescriptor: &ocmdesc.ComponentDescriptor{
			ComponentSpec: ocmdesc.ComponentSpec{
				ObjectMeta: v1.ObjectMeta{
					Name:    cv.Spec.Component,
					Version: "v0.0.1",
					Labels: []v1.Label{
						{
							Name:    "ocm.software/label-value",
							Value:   []byte("\"string-value\""),
							Version: "1.0.0",
						},
						{
							Name:    "ocm.software/label-bool",
							Value:   []byte("true"),
							Version: "1.0.0",
						},
					},
				},
				References: ocmdesc.References{
					{
						ElementMeta: ocmdesc.ElementMeta{
							Name:    "test-ref-1",
							Version: "v0.0.1",
						},
						ComponentName: "github.com/open-component-model/embedded",
					},
				},
			},
		},
	}

	embedded := &ocmfake.Component{
		ComponentDescriptor: &ocmdesc.ComponentDescriptor{
			ComponentSpec: ocmdesc.ComponentSpec{
				ObjectMeta: v1.ObjectMeta{
					Name:    "github.com/open-component-model/embedded",
					Version: "v0.0.1",
					Labels: []v1.Label{
						{
							Name:    "ocm.software/label-value-2",
							Value:   []byte("1.32"),
							Version: "1.0.1",
						},
					},
				},
			},
		},
	}

	fakeOcm := &fakes.MockFetcher{}
	fakeOcm.VerifyComponentReturns(true, nil)
	fakeOcm.GetComponentVersionReturnsForName(embedded.ComponentDescriptor.ComponentSpec.Name, embedded, nil)
	fakeOcm.GetComponentVersionReturnsForName(root.ComponentDescriptor.ComponentSpec.Name, root, nil)
	fakeOcm.VerifyComponentReturns(true, nil)
	fakeOcm.GetLatestComponentVersionReturns("v0.0.1", nil)
	fakeOcm.TransferComponentReturns(nil)
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
	assert.Equal(t, "github.com/open-component-model/internal-test", cv.GetRepositoryURL())
	assert.True(t, conditions.IsTrue(cv, meta.ReadyCondition))

	t.Log("checking label values")
	nns := types.NamespacedName{Name: cv.Status.ComponentDescriptor.ComponentDescriptorRef.Name, Namespace: cv.Status.ComponentDescriptor.ComponentDescriptorRef.Namespace}
	cd := &v1alpha1.ComponentDescriptor{}
	err = client.Get(context.Background(), nns, cd)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"ocm.software/label-value": "string-value",
		"ocm.software/label-bool":  "true",
	}, cd.Labels)

	nns = types.NamespacedName{Name: cv.Status.ComponentDescriptor.References[0].ComponentDescriptorRef.Name, Namespace: cv.Namespace}
	err = client.Get(context.Background(), nns, cd)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"ocm.software/label-value-2": "1.32",
	}, cd.Labels)

	// the second call to GetComponentVersion should fetch it from the new place
	args := fakeOcm.GetComponentVersionCallingArgumentsOnCall(1)
	assert.Equal(t, "github.com/open-component-model/internal-test", args[0])

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
	cv.Spec.Version.Semver = "invalid"
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
	require.NoError(t, err)

	t.Log("verifying updated object status")
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      cv.Name,
		Namespace: cv.Namespace,
	}, cv)
	require.NoError(t, err)

	assert.True(t, conditions.IsFalse(cv, meta.ReadyCondition))

	close(recorder.Events)
	found, event := false, ""
	for e := range recorder.Events {
		fmt.Println(e)
		if strings.Contains(e, v1alpha1.CheckVersionFailedReason) {
			found, event = true, e
			break
		}
	}
	assert.True(t, found)
	assert.Contains(t, event, v1alpha1.CheckVersionFailedReason)
	assert.Contains(t, event, fmt.Sprintf("kind=%s", v1alpha1.ComponentVersionKind))
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
			description:       "using an older version should work if the current constraint disallows the current greater version",
			givenVersion:      "!=0.0.3",
			reconciledVersion: "0.0.3",
			latestVersion:     "0.0.2",
			expectedUpdate:    true,
		},
		{
			description:       "using a newer version should work if the current constraint disallows the current greater version",
			givenVersion:      "!=0.0.3",
			reconciledVersion: "0.0.3",
			latestVersion:     "0.0.4",
			expectedUpdate:    true,
		},
		{
			description:       "make sure update doesn't occur when versions match",
			givenVersion:      ">=0.0.3",
			reconciledVersion: "0.0.4",
			latestVersion:     "0.0.4",
			expectedUpdate:    false,
		},
	}
	for i, tt := range semverTests {
		t.Run(fmt.Sprintf("%d: %s", i, tt.description), func(t *testing.T) {
			t.Helper()

			obj := DefaultComponent.DeepCopy()
			obj.Spec.Version.Semver = tt.givenVersion
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
				if tt.expectedUpdate {
					assert.Contains(t, e, "Version check succeeded, found latest")
				}
			}
		})
	}
}
