// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/patch"
	rreconcile "github.com/fluxcd/pkg/runtime/reconcile"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/open-component-model/ocm-controller/pkg/cache"
	"github.com/open-component-model/ocm-controller/pkg/metrics"
	"github.com/open-component-model/ocm-controller/pkg/status"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	kuberecorder "k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/pkg/ocm"
)

const (
	snapshotFinalizer = "finalizers.snapshot.ocm.software"
	httpsScheme       = "https"
	insecureScheme    = "http"
)

// SnapshotReconciler reconciles a Snapshot object.
type SnapshotReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	kuberecorder.EventRecorder
	RegistryServiceName string

	Cache cache.Cache

	// InsecureSkipVerify if set, snapshot URL will be http instead of https.
	InsecureSkipVerify bool
}

//+kubebuilder:rbac:groups=delivery.ocm.software,resources=snapshots,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=delivery.ocm.software,resources=snapshots/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=delivery.ocm.software,resources=snapshots/finalizers,verbs=update

// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// SetupWithManager sets up the controller with the Manager.
func (r *SnapshotReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Snapshot{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(r)
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *SnapshotReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	obj := &v1alpha1.Snapshot{}
	if err = r.Client.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to get component object: %w", err)
	}

	if obj.GetDeletionTimestamp() != nil {
		if !controllerutil.ContainsFinalizer(obj, snapshotFinalizer) {
			return ctrl.Result{}, nil
		}

		if err = r.reconcileDeleteSnapshot(ctx, obj); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
		}

		return ctrl.Result{}, nil
	}

	if obj.Spec.Suspend {
		return ctrl.Result{}, nil
	}

	patchHelper := patch.NewSerialPatcher(obj, r.Client)

	// AddFinalizer is not present already.
	controllerutil.AddFinalizer(obj, snapshotFinalizer)

	// Always attempt to patch the object and status after each reconciliation.
	defer func() {
		if derr := status.UpdateStatus(ctx, patchHelper, obj, r.EventRecorder, 0); derr != nil {
			err = errors.Join(err, derr)
		}

		if err != nil {
			metrics.SnapshotReconcileFailed.WithLabelValues(obj.Name).Inc()
		}
	}()

	// Starts the progression by setting ReconcilingCondition.
	// This will be checked in defer.
	// Should only be deleted on a success.
	rreconcile.ProgressiveStatus(false, obj, meta.ProgressingReason, "reconciliation in progress for snapshot: %s", obj.Name)

	name, err := ocm.ConstructRepositoryName(obj.Spec.Identity)
	if err != nil {
		err = fmt.Errorf("failed to construct name: %w", err)
		status.MarkNotReady(r.EventRecorder, obj, v1alpha1.CreateRepositoryNameReason, err.Error())

		return ctrl.Result{}, err
	}

	obj.Status.LastReconciledDigest = obj.Spec.Digest
	obj.Status.LastReconciledTag = obj.Spec.Tag

	scheme := httpsScheme
	if r.InsecureSkipVerify {
		scheme = insecureScheme
	}
	obj.Status.RepositoryURL = fmt.Sprintf("%s://%s/%s", scheme, r.RegistryServiceName, name)

	msg := fmt.Sprintf("Snapshot with name '%s' is ready", obj.Name)
	status.MarkReady(r.EventRecorder, obj, msg)
	metrics.SnapshotReconcileSuccess.WithLabelValues(obj.Name).Inc()

	return ctrl.Result{}, nil
}

// reconcileDeleteSnapshot removes the cached data that the snapshot was associated with if it exists.
func (r *SnapshotReconciler) reconcileDeleteSnapshot(ctx context.Context, obj *v1alpha1.Snapshot) error {
	patchHelper, err := patch.NewHelper(obj, r.Client)
	if err != nil {
		return fmt.Errorf("failed to reconcile delete: %w", err)
	}

	name, err := ocm.ConstructRepositoryName(obj.Spec.Identity)
	if err != nil {
		return fmt.Errorf("failed to construct name: %w", err)
	}

	if err := r.Cache.DeleteData(ctx, name, obj.Spec.Tag); err != nil {
		var terr *transport.Error
		if !errors.As(err, &terr) {
			return fmt.Errorf("failure was not a transport error during data deletion: %w", err)
		}

		if terr.StatusCode != http.StatusNotFound && !isUnknownManifestError(terr.Errors) {
			return fmt.Errorf("failed to delete data: %w", err)
		}
	}

	controllerutil.RemoveFinalizer(obj, snapshotFinalizer)

	return patchHelper.Patch(ctx, obj)
}

func isUnknownManifestError(errors []transport.Diagnostic) bool {
	for _, e := range errors {
		if e.Code == transport.ManifestUnknownErrorCode {
			return true
		}
	}

	return false
}
