package controllers

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
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
		if strings.Contains(e, "Reconciliation finished, next run in") {
			event = e
			break
		}
	}
	assert.Contains(t, event, "Reconciliation finished, next run in")
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
		if strings.Contains(e, "Reconciliation finished, next run in") {
			event = e
			break
		}
	}
	assert.Contains(t, event, "Reconciliation finished, next run in")
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
	for _, tt := range []struct {
		name              string
		percentage        uint
		update            bool
		checkFails        bool
		verificationFails bool
		repositoryFails   bool
		suspended         bool
	}{
		{name: "already reconciled", percentage: 5},
		{name: "successful update", percentage: 5, update: true},
		{name: "version check failure", percentage: 5, checkFails: true},
		{name: "verification failure", percentage: 5, update: true, verificationFails: true},
		{name: "error retry", percentage: 5, update: true, repositoryFails: true},
		{name: "suspended", percentage: 5, suspended: true},
		{name: "disabled"},
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
			reconciler := ComponentVersionReconciler{
				Client:                   env.FakeKubeClient(WithObjects(obj)),
				Scheme:                   env.scheme,
				EventRecorder:            recorder,
				OCMClient:                fakeOcm,
				IntervalJitterPercentage: tt.percentage,
			}
			result, err := reconciler.Reconcile(t.Context(), ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      obj.Name,
					Namespace: obj.Namespace,
				},
			})
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
			if tt.percentage == 0 {
				require.Equal(t, time.Minute, result.RequeueAfter)
			} else {
				require.GreaterOrEqual(t, result.RequeueAfter, 57*time.Second)
				require.LessOrEqual(t, result.RequeueAfter, 63*time.Second)
			}
			if !tt.checkFails && !tt.verificationFails {
				close(recorder.Events)
				var finishedEvent string
				for event := range recorder.Events {
					if strings.Contains(event, "Reconciliation finished, next run in") {
						finishedEvent = event
					}
				}
				require.Contains(t, finishedEvent, "Reconciliation finished, next run in "+result.RequeueAfter.String())
			}
		})
	}
}

func TestJitterInterval(t *testing.T) {
	for _, tt := range []struct {
		name       string
		interval   time.Duration
		percentage uint
		minimum    time.Duration
		maximum    time.Duration
	}{
		{name: "disabled", interval: time.Minute, minimum: time.Minute, maximum: time.Minute},
		{name: "five percent", interval: time.Minute, percentage: 5, minimum: 57 * time.Second, maximum: 63 * time.Second},
		{name: "largest percentage", interval: time.Second, percentage: 99, minimum: 10 * time.Millisecond, maximum: 1990 * time.Millisecond},
		{name: "zero interval", percentage: 5},
		{name: "negative interval", interval: -time.Second, percentage: 5, minimum: -time.Second, maximum: -time.Second},
		{name: "smallest interval", interval: time.Nanosecond, percentage: 99, minimum: time.Nanosecond, maximum: time.Nanosecond},
		{name: "largest interval", interval: time.Duration(math.MaxInt64), percentage: 99, minimum: 1, maximum: time.Duration(math.MaxInt64)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for range 100 {
				actual := jitterInterval(tt.interval, tt.percentage)
				require.GreaterOrEqual(t, actual, tt.minimum)
				require.LessOrEqual(t, actual, tt.maximum)
			}
		})
	}
}

func TestJitterIntervalConcurrent(t *testing.T) {
	var workers sync.WaitGroup
	for range 8 {
		workers.Go(func() {
			for range 100 {
				actual := jitterInterval(time.Minute, 5)
				if actual < 57*time.Second || actual > 63*time.Second {
					t.Errorf("jittered interval out of range: %s", actual)
				}
			}
		})
	}
	workers.Wait()
}

func TestJitterIntervalSpreadsPolls(t *testing.T) {
	var earlier, later bool
	for range 128 {
		actual := jitterInterval(time.Minute, 5)
		earlier = earlier || actual < time.Minute
		later = later || actual > time.Minute
	}
	require.True(t, earlier, "jitter should schedule some polls before the base interval")
	require.True(t, later, "jitter should schedule some polls after the base interval")
}
