// Copyright 2022.
// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"errors"
	"fmt"

	"github.com/fluxcd/pkg/runtime/patch"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	rreconcile "github.com/fluxcd/pkg/runtime/reconcile"
	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/pkg/cache"
	"github.com/open-component-model/ocm-controller/pkg/component"
	"github.com/open-component-model/ocm-controller/pkg/ocm"
)

// ResourceReconciler reconciles a Resource object
type ResourceReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	OCMClient ocm.FetchVerifier
	Cache     cache.Cache
}

//+kubebuilder:rbac:groups=delivery.ocm.software,resources=resources,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=delivery.ocm.software,resources=resources/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=delivery.ocm.software,resources=resources/finalizers,verbs=update

// SetupWithManager sets up the controller with the Manager.
func (r *ResourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Resource{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(r)
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ResourceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var (
		retErr error
		result = &ctrl.Result{}
	)

	log := log.FromContext(ctx).WithName("resource-controller")

	log.Info("starting resource reconcile loop")
	obj := &v1alpha1.Resource{}
	if err := r.Client.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		retErr = fmt.Errorf("failed to get resource object: %w", err)
		return ctrl.Result{}, retErr
	}

	snapshot, err := CheckIfSnapshotExists(ctx, r.Client, obj.Spec.SnapshotTemplate.Name, obj.Namespace)
	if err != nil {
		retErr = errors.Join(retErr, err)
		return ctrl.Result{}, retErr
	}

	if snapshot != nil && conditions.IsTrue(snapshot, meta.ReadyCondition) {
		log.Info("snapshot found and is done for resource, re-queueing", "snapshot", obj.Spec.SnapshotTemplate.Name)
		return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, nil
	}

	patchHelper, err := patch.NewHelper(obj, r.Client)
	if err != nil {
		retErr = errors.Join(retErr, err)
		return ctrl.Result{}, retErr
	}

	// Always attempt to patch the object and status after each reconciliation.
	defer func() {
		// Patching has not been set up, or the controller errored earlier.
		if patchHelper == nil {
			return
		}

		if condition := conditions.Get(obj, meta.StalledCondition); condition != nil && condition.Status == metav1.ConditionTrue {
			conditions.Delete(obj, meta.ReconcilingCondition)
		}

		// Check if it's a successful reconciliation.
		// We don't set Requeue in case of error, so we can safely check for Requeue.
		if result.RequeueAfter == obj.GetRequeueAfter() && !result.Requeue && retErr == nil {
			// Remove the reconciling condition if it's set.
			conditions.Delete(obj, meta.ReconcilingCondition)

			// Set the return err as the ready failure message is the resource is not ready, but also not reconciling or stalled.
			if ready := conditions.Get(obj, meta.ReadyCondition); ready != nil && ready.Status == metav1.ConditionFalse && !conditions.IsStalled(obj) {
				retErr = errors.New(conditions.GetMessage(obj, meta.ReadyCondition))
			}
		}

		// If still reconciling then reconciliation did not succeed, set to ProgressingWithRetry to
		// indicate that reconciliation will be retried.
		if conditions.IsReconciling(obj) {
			reconciling := conditions.Get(obj, meta.ReconcilingCondition)
			reconciling.Reason = meta.ProgressingWithRetryReason
			conditions.Set(obj, reconciling)
		}

		// If not reconciling or stalled than mark Ready=True
		if !conditions.IsReconciling(obj) && !conditions.IsStalled(obj) &&
			retErr == nil && result.RequeueAfter == obj.GetRequeueAfter() {
			conditions.MarkTrue(obj, meta.ReadyCondition, meta.SucceededReason, "Reconciliation success")
		}

		// Set status observed generation option if the object is stalled or ready.
		if conditions.IsStalled(obj) || conditions.IsReady(obj) {
			obj.Status.ObservedGeneration = obj.Generation
		}

		if err := patchHelper.Patch(ctx, obj); err != nil {
			retErr = errors.Join(retErr, err)
		}
	}()

	log.Info("found resource", "resource", obj)

	*result, retErr = r.reconcile(ctx, obj)
	return *result, retErr
}

func (r *ResourceReconciler) reconcile(ctx context.Context, obj *v1alpha1.Resource) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithName("resource-controller")

	rreconcile.ProgressiveStatus(false, obj, meta.ProgressingReason, "reconciliation in progress")

	if obj.Generation != obj.Status.ObservedGeneration {
		rreconcile.ProgressiveStatus(false, obj, meta.ProgressingReason,
			"processing object: new generation %d -> %d", obj.Status.ObservedGeneration, obj.Generation)
	}

	log.Info("finding component ref", "resource", obj)

	// read component version
	cdvKey := types.NamespacedName{
		Name:      obj.Spec.ComponentVersionRef.Name,
		Namespace: obj.Spec.ComponentVersionRef.Namespace,
	}

	componentVersion := &v1alpha1.ComponentVersion{}
	if err := r.Get(ctx, cdvKey, componentVersion); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, nil
		}

		err = fmt.Errorf("failed to get component version: %w", err)
		conditions.MarkStalled(obj, v1alpha1.ComponentVersionInvalidReason, err.Error())
		conditions.MarkFalse(obj, meta.ReadyCondition, v1alpha1.ComponentVersionInvalidReason, err.Error())
		return ctrl.Result{}, err
	}

	conditions.Delete(obj, meta.StalledCondition)

	log.Info("got component version", "component version", cdvKey.String())

	reader, digest, err := r.OCMClient.GetResource(ctx, componentVersion, obj.Spec.Resource)
	if err != nil {
		err = fmt.Errorf("failed to get resource: %w", err)
		conditions.MarkFalse(obj, meta.ReadyCondition, v1alpha1.GetResourceFailedReason, err.Error())
		return ctrl.Result{}, err
	}
	defer reader.Close()

	version := "latest"
	if obj.Spec.Resource.Version != "" {
		version = obj.Spec.Resource.Version
	}

	// This is important because THIS is the actual component for our resource. If we used ComponentVersion in the
	// below identity, that would be the top-level component instead of the component that this resource belongs to.
	componentDescriptor, err := component.GetComponentDescriptor(ctx, r.Client, obj.Spec.Resource.ReferencePath, componentVersion.Status.ComponentDescriptor)
	if err != nil {
		err = fmt.Errorf("failed to get component descriptor for resource: %w", err)
		conditions.MarkFalse(obj, meta.ReadyCondition, v1alpha1.GetComponentDescriptorFailedReason, err.Error())
		return ctrl.Result{}, err
	}

	if componentDescriptor == nil {
		err := fmt.Errorf("couldn't find component descriptor for reference '%s' or any root components", obj.Spec.Resource.ReferencePath)
		conditions.MarkFalse(obj, meta.ReadyCondition, v1alpha1.ComponentDescriptorNotFoundReason, err.Error())
		// Mark stalled because we can't do anything until the component descriptor is available. Likely requires some sort of manual intervention.
		conditions.MarkStalled(obj, v1alpha1.ComponentDescriptorNotFoundReason, err.Error())
		return ctrl.Result{}, err
	}

	conditions.Delete(obj, meta.StalledCondition)

	identity := v1alpha1.Identity{
		v1alpha1.ComponentNameKey:    componentDescriptor.Name,
		v1alpha1.ComponentVersionKey: componentDescriptor.Spec.Version,
		v1alpha1.ResourceNameKey:     obj.Spec.Resource.Name,
		v1alpha1.ResourceVersionKey:  version,
	}
	for k, v := range obj.Spec.Resource.ExtraIdentity {
		identity[k] = v
	}

	// How would I use this snapshot from the Localizer?
	snapshotCR := &v1alpha1.Snapshot{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: obj.GetNamespace(),
			Name:      obj.Spec.SnapshotTemplate.Name,
		},
	}

	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, snapshotCR, func() error {
		if snapshotCR.ObjectMeta.CreationTimestamp.IsZero() {
			if err := controllerutil.SetOwnerReference(obj, snapshotCR, r.Scheme); err != nil {
				return fmt.Errorf("failed to set owner to snapshot object: %w", err)
			}
		}
		snapshotCR.Spec = v1alpha1.SnapshotSpec{
			Identity:         identity,
			CreateFluxSource: obj.Spec.SnapshotTemplate.CreateFluxSource,
			Digest:           digest,
			Tag:              version,
		}
		return nil
	})
	if err != nil {
		err = fmt.Errorf("failed to create or update snapshot: %w", err)
		conditions.MarkFalse(obj, meta.ReadyCondition, v1alpha1.CreateOrUpdateSnapshotFailedReason, err.Error())
		return ctrl.Result{}, err
	}

	log.Info("successfully pushed resource", "resource", obj.Spec.Resource.Name)
	obj.Status.LastAppliedResourceVersion = obj.Spec.Resource.Version
	obj.Status.ObservedGeneration = obj.GetGeneration()

	log.Info("successfully reconciled resource", "name", obj.GetName())

	// Remove any stale Ready condition, most likely False, set above. Its value
	// is derived from the overall result of the reconciliation in the deferred
	// block at the very end.
	conditions.Delete(obj, meta.ReadyCondition)

	return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, nil
}
