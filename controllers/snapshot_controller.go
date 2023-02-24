// Copyright 2022.
// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/patch"
	"github.com/fluxcd/source-controller/api/v1beta2"
	"github.com/open-component-model/ocm-controller/pkg/cache"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	deliveryv1alpha1 "github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/pkg/ocm"
)

const (
	snapshotFinalizer = "finalizers.snapshot.ocm.software"
)

// SnapshotReconciler reconciles a Snapshot object
type SnapshotReconciler struct {
	client.Client
	Scheme              *runtime.Scheme
	RegistryServiceName string

	Cache cache.Cache
}

//+kubebuilder:rbac:groups=delivery.ocm.software,resources=snapshots,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=delivery.ocm.software,resources=snapshots/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=delivery.ocm.software,resources=snapshots/finalizers,verbs=update

// +kubebuilder:rbac:groups=source.toolkit.fluxcd.io,resources=ocirepositories,verbs=get;list;watch;create;update;patch;delete

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
		}

		if err := patchHelper.Patch(ctx, obj); err != nil {
			retErr = errors.Join(retErr, err)
		}
	}()

	name, err := ocm.ConstructRepositoryName(obj.Spec.Identity)
	if err != nil {
		conditions.MarkFalse(obj, meta.ReadyCondition, v1alpha1.CreateRepositoryNameReason, err.Error())
		return ctrl.Result{}, fmt.Errorf("failed to construct name: %w", err)
	}

	if obj.Spec.CreateFluxSource {
		log.Info("reconciling flux oci source for snapshot")

		ociRepoCR := &v1beta2.OCIRepository{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: obj.GetNamespace(),
				Name:      obj.GetName(),
			},
		}

		_, err = controllerutil.CreateOrUpdate(ctx, r.Client, ociRepoCR, func() error {
			if ociRepoCR.ObjectMeta.CreationTimestamp.IsZero() {
				if err := controllerutil.SetOwnerReference(obj, ociRepoCR, r.Scheme); err != nil {
					return fmt.Errorf("failed to set owner reference on oci repository source: %w", err)
				}
			}
			ociRepoCR.Spec = v1beta2.OCIRepositorySpec{
				Interval: metav1.Duration{Duration: time.Hour},
				Insecure: true,
				URL:      fmt.Sprintf("oci://%s/%s", r.RegistryServiceName, name),
				Reference: &v1beta2.OCIRepositoryRef{
					Tag: obj.Spec.Tag,
				},
			}
			return nil
		})
		if err != nil {
			log.Error(err, "failed to create or update oci repository")
			conditions.MarkFalse(obj, meta.ReadyCondition, v1alpha1.CreateOrUpdateOCIRepositoryFailedReason, err.Error())
			conditions.MarkStalled(obj, v1alpha1.CreateOrUpdateOCIRepositoryFailedReason, err.Error())
			retErr = nil
			return ctrl.Result{}, nil
		}
	}

	obj.Status.LastReconciledDigest = obj.Spec.Digest
	obj.Status.LastReconciledTag = obj.Spec.Tag
	obj.Status.RepositoryURL = fmt.Sprintf("http://%s/%s", r.RegistryServiceName, name)
	conditions.MarkTrue(obj, meta.ReadyCondition, meta.SucceededReason, "Snapshot with name '%s' is ready", obj.Name)
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
		if !strings.Contains(err.Error(), "MANIFEST_UNKNOWN") {
			return fmt.Errorf("failed to remove cached data: %w", err)
		}
	}

	controllerutil.RemoveFinalizer(obj, snapshotFinalizer)

	return patchHelper.Patch(ctx, obj)
}
