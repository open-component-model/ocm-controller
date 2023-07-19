// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	ocmmetav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/pkg/cache/fakes"
)

func TestSnapshotReconciler(t *testing.T) {
	snapshot := &v1alpha1.Snapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-snapshot",
			Namespace: "default",
		},
		Spec: v1alpha1.SnapshotSpec{
			Identity: ocmmetav1.Identity{
				v1alpha1.ComponentNameKey:    "component-name",
				v1alpha1.ComponentVersionKey: "v0.0.1",
				v1alpha1.ResourceNameKey:     "resource-name",
				v1alpha1.ResourceVersionKey:  "v0.0.5",
			},
			Digest: "digest-1",
			Tag:    "1234",
		},
	}
	client := env.FakeKubeClient(WithObjects(snapshot))
	fakeCache := &fakes.FakeCache{}
	recorder := record.NewFakeRecorder(32)

	sr := SnapshotReconciler{
		Client:              client,
		Scheme:              env.scheme,
		RegistryServiceName: "127.0.0.1:5000",
		EventRecorder:       recorder,
		Cache:               fakeCache,
	}
	result, err := sr.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      snapshot.Name,
			Namespace: snapshot.Namespace,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
	err = client.Get(context.Background(), types.NamespacedName{Name: snapshot.Name, Namespace: snapshot.Namespace}, snapshot)
	require.NoError(t, err)
	assert.True(t, conditions.IsTrue(snapshot, meta.ReadyCondition))
	assert.Equal(t, "digest-1", snapshot.Status.LastReconciledDigest)
	assert.Equal(t, "1234", snapshot.Status.LastReconciledTag)
	assert.Equal(t, "https://127.0.0.1:5000/sha-16038726184537443379", snapshot.Status.RepositoryURL)

	close(recorder.Events)
	event := ""
	for e := range recorder.Events {
		if strings.Contains(e, "Reconciliation finished") {
			event = e
			break
		}
	}
	assert.Contains(t, event, "Reconciliation finished")
}

func TestSnapshotReconcilerDelete(t *testing.T) {
	snapshot := &v1alpha1.Snapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-snapshot",
			Namespace: "default",
			DeletionTimestamp: &metav1.Time{
				Time: time.Now(),
			},
		},
		Spec: v1alpha1.SnapshotSpec{
			Identity: ocmmetav1.Identity{
				v1alpha1.ComponentNameKey:    "component-name",
				v1alpha1.ComponentVersionKey: "v0.0.1",
				v1alpha1.ResourceNameKey:     "resource-name",
				v1alpha1.ResourceVersionKey:  "v0.0.5",
			},
			Digest: "digest-1",
			Tag:    "1234",
		},
	}
	controllerutil.AddFinalizer(snapshot, snapshotFinalizer)
	client := env.FakeKubeClient(WithObjects(snapshot))
	fakeCache := &fakes.FakeCache{}
	recorder := record.NewFakeRecorder(32)

	sr := SnapshotReconciler{
		Client:              client,
		Scheme:              env.scheme,
		RegistryServiceName: "127.0.0.1:5000",
		EventRecorder:       recorder,
		Cache:               fakeCache,
	}
	result, err := sr.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      snapshot.Name,
			Namespace: snapshot.Namespace,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
	err = client.Get(context.Background(), types.NamespacedName{Name: snapshot.Name, Namespace: snapshot.Namespace}, snapshot)
	assert.True(t, apierror.IsNotFound(err))
}

func TestSnapshotReconcilerDeleteFails(t *testing.T) {
	snapshot := &v1alpha1.Snapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-snapshot",
			Namespace: "default",
			DeletionTimestamp: &metav1.Time{
				Time: time.Now(),
			},
		},
		Spec: v1alpha1.SnapshotSpec{
			Identity: ocmmetav1.Identity{
				v1alpha1.ComponentNameKey:    "component-name",
				v1alpha1.ComponentVersionKey: "v0.0.1",
				v1alpha1.ResourceNameKey:     "resource-name",
				v1alpha1.ResourceVersionKey:  "v0.0.5",
			},
			Digest: "digest-1",
			Tag:    "1234",
		},
	}
	controllerutil.AddFinalizer(snapshot, snapshotFinalizer)
	client := env.FakeKubeClient(WithObjects(snapshot))
	fakeCache := &fakes.FakeCache{}
	fakeCache.DeleteDataReturns(errors.New("nope"))
	recorder := record.NewFakeRecorder(32)

	sr := SnapshotReconciler{
		Client:              client,
		Scheme:              env.scheme,
		RegistryServiceName: "127.0.0.1:5000",
		EventRecorder:       recorder,
		Cache:               fakeCache,
	}
	_, err := sr.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      snapshot.Name,
			Namespace: snapshot.Namespace,
		},
	})
	require.Error(t, err)
	err = client.Get(context.Background(), types.NamespacedName{Name: snapshot.Name, Namespace: snapshot.Namespace}, snapshot)
	require.NoError(t, err)
	assert.False(t, fakeCache.DeleteDataWasNotCalled())
	assert.True(t, controllerutil.ContainsFinalizer(snapshot, snapshotFinalizer))
}

func TestSnapshotReconcilerDeleteFailsWithManifestNotFound(t *testing.T) {
	snapshot := &v1alpha1.Snapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-snapshot",
			Namespace: "default",
			DeletionTimestamp: &metav1.Time{
				Time: time.Now(),
			},
		},
		Spec: v1alpha1.SnapshotSpec{
			Identity: ocmmetav1.Identity{
				v1alpha1.ComponentNameKey:    "component-name",
				v1alpha1.ComponentVersionKey: "v0.0.1",
				v1alpha1.ResourceNameKey:     "resource-name",
				v1alpha1.ResourceVersionKey:  "v0.0.5",
			},
			Digest: "digest-1",
			Tag:    "1234",
		},
	}
	controllerutil.AddFinalizer(snapshot, snapshotFinalizer)
	client := env.FakeKubeClient(WithObjects(snapshot))
	fakeCache := &fakes.FakeCache{}
	fakeCache.DeleteDataReturns(&transport.Error{
		Errors: []transport.Diagnostic{
			{
				Code: transport.ManifestUnknownErrorCode,
			},
		},
		StatusCode: 0,
		Request:    nil,
	})
	recorder := record.NewFakeRecorder(32)

	sr := SnapshotReconciler{
		Client:              client,
		Scheme:              env.scheme,
		RegistryServiceName: "127.0.0.1:5000",
		EventRecorder:       recorder,
		Cache:               fakeCache,
	}
	_, err := sr.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      snapshot.Name,
			Namespace: snapshot.Namespace,
		},
	})
	require.NoError(t, err)
	err = client.Get(context.Background(), types.NamespacedName{Name: snapshot.Name, Namespace: snapshot.Namespace}, snapshot)
	assert.True(t, apierror.IsNotFound(err))
}

func TestSnapshotReconcilerDeleteFailsWithNotFoundStatusCode(t *testing.T) {
	snapshot := &v1alpha1.Snapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-snapshot",
			Namespace: "default",
			DeletionTimestamp: &metav1.Time{
				Time: time.Now(),
			},
		},
		Spec: v1alpha1.SnapshotSpec{
			Identity: ocmmetav1.Identity{
				v1alpha1.ComponentNameKey:    "component-name",
				v1alpha1.ComponentVersionKey: "v0.0.1",
				v1alpha1.ResourceNameKey:     "resource-name",
				v1alpha1.ResourceVersionKey:  "v0.0.5",
			},
			Digest: "digest-1",
			Tag:    "1234",
		},
	}
	controllerutil.AddFinalizer(snapshot, snapshotFinalizer)
	client := env.FakeKubeClient(WithObjects(snapshot))
	fakeCache := &fakes.FakeCache{}
	fakeCache.DeleteDataReturns(&transport.Error{
		Errors: []transport.Diagnostic{
			{
				Code: transport.ManifestBlobUnknownErrorCode,
			},
		},
		StatusCode: http.StatusNotFound,
		Request:    nil,
	})
	recorder := record.NewFakeRecorder(32)

	sr := SnapshotReconciler{
		Client:              client,
		Scheme:              env.scheme,
		RegistryServiceName: "127.0.0.1:5000",
		EventRecorder:       recorder,
		Cache:               fakeCache,
	}
	_, err := sr.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      snapshot.Name,
			Namespace: snapshot.Namespace,
		},
	})
	require.NoError(t, err)
	err = client.Get(context.Background(), types.NamespacedName{Name: snapshot.Name, Namespace: snapshot.Namespace}, snapshot)
	assert.True(t, apierror.IsNotFound(err))
}
