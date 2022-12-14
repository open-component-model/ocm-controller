// Copyright 2022.
// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"fmt"

	"github.com/fluxcd/pkg/runtime/conditions"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

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

// SetupWithManager sets up the controller with the Manager.
func (r *SnapshotReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&deliveryv1alpha1.Snapshot{}).
		Complete(r)
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *SnapshotReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithName("snapshot-reconcile")

	log.Info("reconciling snapshot")

	obj := &v1alpha1.Snapshot{}
	if err := r.Client.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get component object: %w", err)
	}

	patchHelper, err := patch.NewHelper(obj, r.Client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create patch helper: %w", err)
	}

	conditions.MarkTrue(obj, v1alpha1.SnapshotReady, v1alpha1.SnapshotReadyReason, "Snapshot with name '%s' is ready", obj.Name)
	name, err := ocm.ConstructRepositoryName(obj.Spec.Identity)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to construct name: %w", err)
	}
	obj.Status.RepositoryURL = fmt.Sprintf("http://%s/%s@%s", r.RegistryServiceName, name, obj.Status.Digest)

	if err := patchHelper.Patch(ctx, obj); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to patch resource: %w", err)
	}

	return ctrl.Result{}, nil
}
