// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"errors"
	"fmt"
	"strings"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/patch"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	kuberecorder "k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	deliveryv1alpha1 "github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/controllers/sources"
	"github.com/open-component-model/ocm-controller/pkg/cache"
	"github.com/open-component-model/ocm-controller/pkg/event"
	"github.com/open-component-model/ocm-controller/pkg/ocm"
)

const (
	snapshotFinalizer = "finalizers.snapshot.ocm.software"
)

// SnapshotReconciler reconciles a Snapshot object
type SnapshotReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	kuberecorder.EventRecorder
	RegistryServiceName string

	Cache         cache.Cache
	SourceCreator sources.FluxSource
}

//+kubebuilder:rbac:groups=delivery.ocm.software,resources=snapshots,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=delivery.ocm.software,resources=snapshots/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=delivery.ocm.software,resources=snapshots/finalizers,verbs=update

// +kubebuilder:rbac:groups=source.toolkit.fluxcd.io,resources=ocirepositories,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=source.toolkit.fluxcd.io,resources=helmrepositories,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// SetupWithManager sets up the controller with the Manager.
func (r *SnapshotReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&deliveryv1alpha1.Snapshot{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(r)
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *SnapshotReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var retErr error
	log := log.FromContext(ctx).WithName("snapshot-reconcile")

	log.Info("reconciling snapshot")

	obj := &v1alpha1.Snapshot{}
	if err := r.Client.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		retErr = fmt.Errorf("failed to get component object: %w", err)
		return ctrl.Result{}, retErr
	}

	if obj.GetDeletionTimestamp() != nil {
		if !controllerutil.ContainsFinalizer(obj, snapshotFinalizer) {
			return ctrl.Result{}, nil
		}

		if err := r.reconcileDeleteSnapshot(ctx, obj); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
		}

		return ctrl.Result{}, nil
	}

	patchHelper, err := patch.NewHelper(obj, r.Client)
	if err != nil {
		retErr = errors.Join(retErr, err)
		return ctrl.Result{}, retErr
	}

	// AddFinalizer is not present already.
	controllerutil.AddFinalizer(obj, snapshotFinalizer)

	// Always attempt to patch the object and status after each reconciliation.
	defer func() {
		// Patching has not been set up, or the controller errored earlier.
		if patchHelper == nil {
			return
		}

		// Set status observed generation option if the object is stalled or ready.
		if conditions.IsStalled(obj) || conditions.IsReady(obj) {
			obj.Status.ObservedGeneration = obj.Generation
			event.New(r.EventRecorder, obj, eventv1.EventSeverityInfo, "Reconciliation finished",
				map[string]string{v1alpha1.GroupVersion.Group + "/snapshot_digest": obj.Status.LastReconciledDigest})
		}

		if err := patchHelper.Patch(ctx, obj); err != nil {
			retErr = errors.Join(retErr, err)
		}
	}()

	name, err := ocm.ConstructRepositoryName(obj.Spec.Identity)
	if err != nil {
		conditions.MarkFalse(obj, meta.ReadyCondition, v1alpha1.CreateRepositoryNameReason, err.Error())
		event.New(r.EventRecorder, obj, eventv1.EventSeverityError, err.Error(), nil)
		return ctrl.Result{}, fmt.Errorf("failed to construct name: %w", err)
	}

	if obj.Spec.CreateFluxSource {
		if err := r.SourceCreator.CreateSource(ctx, obj, r.RegistryServiceName, name, obj.GetContentType()); err != nil {
			msg := "failed to create or update source repository object"
			conditions.MarkFalse(obj, meta.ReadyCondition, v1alpha1.CreateOrUpdateOCIRepositoryFailedReason, err.Error())
			conditions.MarkStalled(obj, v1alpha1.CreateOrUpdateOCIRepositoryFailedReason, err.Error())
			event.New(r.EventRecorder, obj, eventv1.EventSeverityError, msg, nil)
			retErr = fmt.Errorf("failed to create flux source: %w", err)
			return ctrl.Result{}, retErr
		}
	}

	if obj.Spec.DuplicateTagToTag != "" {
		reader, err := r.Cache.FetchDataByDigest(ctx, name, obj.Spec.Digest)
		if err != nil {
			msg := "failed to fetch data by digest"

			conditions.MarkFalse(obj, meta.ReadyCondition, v1alpha1.GetResourceFailedReason, err.Error())
			event.New(r.EventRecorder, obj, eventv1.EventSeverityError, msg, nil)
			retErr = fmt.Errorf("failed to fetch resource: %w", err)

			return ctrl.Result{}, retErr
		}

		if _, err := r.Cache.PushData(ctx, reader, name, obj.Spec.DuplicateTagToTag); err != nil {
			msg := "failed to push data to new tag " + obj.Spec.DuplicateTagToTag

			conditions.MarkFalse(obj, meta.ReadyCondition, v1alpha1.CacheCreateOperationFailedReason, err.Error())
			event.New(r.EventRecorder, obj, eventv1.EventSeverityError, msg, nil)
			retErr = fmt.Errorf("failed to push data: %w", err)

			return ctrl.Result{}, retErr
		}
	}

	obj.Status.LastReconciledDigest = obj.Spec.Digest
	obj.Status.LastReconciledTag = obj.Spec.Tag
	obj.Status.RepositoryURL = fmt.Sprintf("https://%s/%s", r.RegistryServiceName, name)
	msg := fmt.Sprintf("Snapshot with name '%s' is ready", obj.Name)
	conditions.MarkTrue(obj, meta.ReadyCondition, meta.SucceededReason, msg)
	log.Info("snapshot successfully reconciled", "snapshot", klog.KObj(obj))

	return ctrl.Result{}, nil
}

// reconcileDeleteSnapshot removes the cached data that the snapshot was associated with if it exists.
func (r *SnapshotReconciler) reconcileDeleteSnapshot(ctx context.Context, obj *deliveryv1alpha1.Snapshot) error {
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
		if errors.As(err, &terr) && containsManifestNotFoundError(terr.Errors) || strings.Contains(err.Error(), "404 Not Found") {
			controllerutil.RemoveFinalizer(obj, snapshotFinalizer)
			return patchHelper.Patch(ctx, obj)
		}

		return fmt.Errorf("failed to delete data: %w", err)
	}

	controllerutil.RemoveFinalizer(obj, snapshotFinalizer)

	return patchHelper.Patch(ctx, obj)
}

// "error": "failed to remove finalizer: failed to delete data: failed to fetch head for reference: HEAD http://registry.ocm-system.svc.cluster.local:5000/v2/sha-6200481511978943855/manifests/0.1.0: unexpected status code 404 Not Found (HEAD responses have no body, use GET for details)"}
func containsManifestNotFoundError(errors []transport.Diagnostic) bool {
	for _, e := range errors {
		if e.Code == transport.ManifestUnknownErrorCode {
			return true
		}
	}

	return false
}
