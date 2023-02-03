// Copyright 2022.
// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/patch"
	"github.com/fluxcd/source-controller/api/v1beta2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
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

// SnapshotReconciler reconciles a Snapshot object
type SnapshotReconciler struct {
	client.Client
	Scheme              *runtime.Scheme
	RegistryServiceName string
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
func (r *SnapshotReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, retErr error) {
	log := log.FromContext(ctx).WithName("snapshot-reconcile")

	log.Info("reconciling snapshot")

	obj := &v1alpha1.Snapshot{}
	if err := r.Client.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			result, retErr = ctrl.Result{}, nil
			return
		}
		result, retErr = ctrl.Result{}, fmt.Errorf("failed to get component object: %w", err)
		return
	}

	// Always attempt to patch the object and status after each reconciliation.
	defer func() {
		// Set status observed generation option if the object is stalled or ready.
		if conditions.IsStalled(obj) || conditions.IsReady(obj) {
			obj.Status.ObservedGeneration = obj.Generation
		}

		patchHelper, err := patch.NewHelper(obj, r.Client)
		if err != nil {
			result, retErr = ctrl.Result{}, err
			return
		}

		if err := patchHelper.Patch(ctx, obj); err != nil {
			if !obj.GetDeletionTimestamp().IsZero() {
				err = kerrors.FilterOut(err, func(e error) bool { return apierrors.IsNotFound(e) })
			}

			retErr = kerrors.NewAggregate([]error{retErr, err})
		}
	}()

	name, err := ocm.ConstructRepositoryName(obj.Spec.Identity)
	if err != nil {
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
			return ctrl.Result{}, fmt.Errorf("failed to create or update component descriptor: %w", err)
		}
	}

	obj.Status.LastReconciledDigest = obj.Spec.Digest
	obj.Status.LastReconciledTag = obj.Spec.Tag
	obj.Status.RepositoryURL = fmt.Sprintf("http://%s/%s", r.RegistryServiceName, name)
	conditions.MarkTrue(obj, meta.ReadyCondition, meta.SucceededReason, "Snapshot with name '%s' is ready", obj.Name)

	return ctrl.Result{}, nil
}
