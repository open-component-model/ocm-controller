package controllers

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/jitter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"

	ocmdesc "ocm.software/ocm/api/ocm/compdesc"
	v1 "ocm.software/ocm/api/ocm/compdesc/meta/v1"

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
	recorder := &events.FakeRecorder{
		Events:  make(chan string, 32),
		Verbose: true,
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
		if strings.Contains(e, "Reconciliation finished, polling interval") {
			event = e
			break
		}
	}
	assert.Contains(t, event, "Reconciliation finished, polling interval")
	assert.Contains(t, event, "delivery.ocm.software/component_version")
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
	recorder := &events.FakeRecorder{
		Events:  make(chan string, 32),
		Verbose: true,
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
		if strings.Contains(e, "Reconciliation finished, polling interval") {
			event = e
			break
		}
	}
	assert.Contains(t, event, "Reconciliation finished, polling interval")
	assert.Contains(t, event, "delivery.ocm.software/component_version")
}

func TestComponentVersionReconcileFailure(t *testing.T) {
	cv := DefaultComponent.DeepCopy()
	cv.Spec.Version.Semver = "invalid"
	client := env.FakeKubeClient(WithObjects(cv))
	recorder := &events.FakeRecorder{
		Events:  make(chan string, 32),
		Verbose: true,
	}

	fakeOcm := &fakes.MockFetcher{}
	fakeOcm.GetLatestComponentVersionReturns("v0.0.1", nil)
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
			recorder := &events.FakeRecorder{
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

func TestComponentVersionPollingJitter(t *testing.T) {
	// Flux initializes its global jitter only once. Use a fresh process for each
	// percentage, including zero, without changing other controller tests.
	const percentageEnv = "OCM_TEST_INTERVAL_JITTER_PERCENTAGE"
	percentageValue := os.Getenv(percentageEnv)
	if percentageValue == "" {
		for _, value := range []string{"0", "5", "15", "99"} {
			t.Run(value+" percent", func(t *testing.T) {
				cmd := exec.Command(os.Args[0], "-test.run=^TestComponentVersionPollingJitter$", "-test.v")
				cmd.Env = append(os.Environ(), percentageEnv+"="+value)
				output, err := cmd.CombinedOutput()
				require.NoError(t, err, string(output))
			})
		}
		return
	}
	percentage, err := strconv.ParseUint(percentageValue, 10, 8)
	require.NoError(t, err)
	options := jitter.IntervalOptions{Percentage: uint8(percentage)}
	require.NoError(t, options.SetGlobalJitter(rand.New(rand.NewSource(1))))

	for _, tt := range []struct {
		name              string
		update            bool
		checkFails        bool
		verificationFails bool
		repositoryFails   bool
		suspended         bool
	}{
		{name: "already reconciled"},
		{name: "successful update", update: true},
		{name: "version check failure", checkFails: true},
		{name: "verification failure", update: true, verificationFails: true},
		{name: "error retry", update: true, repositoryFails: true},
		{name: "suspended", suspended: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			obj := DefaultComponent.DeepCopy()
			obj.Spec.Interval = metav1.Duration{Duration: time.Minute}
			obj.Spec.Version.Semver = ">=1.0.0"
			obj.Spec.Suspend = tt.suspended
			if !tt.update {
				obj.Status.ReconciledVersion = "1.0.0"
			}
			root := &ocmfake.Component{
				Name:    obj.Spec.Component,
				Version: "1.0.0",
				ComponentDescriptor: &ocmdesc.ComponentDescriptor{
					ComponentSpec: ocmdesc.ComponentSpec{
						ObjectMeta: v1.ObjectMeta{
							Name:    obj.Spec.Component,
							Version: "1.0.0",
						},
					},
				},
			}
			fakeOcm := &fakes.MockFetcher{}
			fakeOcm.GetLatestComponentVersionReturns("1.0.0", nil)
			fakeOcm.VerifyComponentReturns(true, nil)
			fakeOcm.GetComponentVersionReturnsForName(root.Name, root, nil)
			if tt.checkFails {
				fakeOcm.GetLatestComponentVersionReturns("", errors.New("no matching version"))
			}
			if tt.verificationFails {
				fakeOcm.VerifyComponentReturns(false, errors.New("invalid signature"))
			}
			if tt.repositoryFails {
				fakeOcm.GetComponentVersionReturnsForName(root.Name, nil, errors.New("repository unavailable"))
			}
			recorder := &events.FakeRecorder{Events: make(chan string, 32)}
			testEnvironment := &testEnv{scheme: env.scheme}
			kubeClient := testEnvironment.FakeKubeClient(WithObjects(obj))
			reconciler := ComponentVersionReconciler{
				Client:        kubeClient,
				Scheme:        testEnvironment.scheme,
				EventRecorder: recorder,
				OCMClient:     fakeOcm,
			}
			// Exercise concurrent access to Flux's shared random generator.
			t.Parallel()
			request := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      obj.Name,
					Namespace: obj.Namespace,
				},
			}
			var previousEvent string
			for range 2 {
				result, err := reconciler.Reconcile(t.Context(), request)
				if tt.repositoryFails {
					require.ErrorContains(t, err, "repository unavailable")
					require.Zero(t, result.RequeueAfter)
					return
				}
				require.NoError(t, err)
				if tt.suspended {
					require.Zero(t, result.RequeueAfter)
					return
				}
				if percentage == 0 {
					require.Equal(t, time.Minute, result.RequeueAfter)
				} else {
					minimum := time.Minute - time.Duration(percentage)*time.Minute/100
					maximum := time.Minute + time.Duration(percentage)*time.Minute/100
					require.GreaterOrEqual(t, result.RequeueAfter, minimum)
					require.LessOrEqual(t, result.RequeueAfter, maximum)
					require.NotEqual(t, time.Minute, result.RequeueAfter)
				}
				if !tt.checkFails && !tt.verificationFails {
					var finishedEvent string
					for len(recorder.Events) > 0 {
						event := <-recorder.Events
						if strings.Contains(event, "Reconciliation finished, polling interval") {
							finishedEvent = event
						}
					}
					require.Contains(t, finishedEvent, "Reconciliation finished, polling interval 1m0s")
					if previousEvent != "" {
						require.Equal(t, previousEvent, finishedEvent, "polling events must remain identical for aggregation")
					}
					previousEvent = finishedEvent
				}
			}
		})
	}
}
