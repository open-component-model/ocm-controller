// Copyright 2022.
// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

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
	log := log.FromContext(ctx).WithName("resource-controller")

	log.Info("starting resource reconcile loop")
	resource := &v1alpha1.Resource{}
	if err := r.Client.Get(ctx, req.NamespacedName, resource); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get resource object: %w", err)
	}
	log.Info("found resource", "resource", resource)

	return r.reconcile(ctx, resource)
}

func (r *ResourceReconciler) reconcile(ctx context.Context, obj *v1alpha1.Resource) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithName("resource-controller")

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

		return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()},
			fmt.Errorf("failed to get component version: %w", err)
	}

	log.Info("got component version", "component version", cdvKey.String())

	// Initialize the patch helper.
	patchHelper, err := patch.NewHelper(obj, r.Client)
	if err != nil {
		return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, err
	}

	reader, digest, err := r.OCMClient.GetResource(ctx, componentVersion, obj.Spec.Resource)
	if err != nil {
		return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, fmt.Errorf("failed to get resource: %w", err)
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
		return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()},
			fmt.Errorf("failed to get component descriptor for resource: %w", err)
	}

	if componentDescriptor == nil {
		return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()},
			fmt.Errorf("couldn't find component descriptor for reference '%s' or any root components", obj.Spec.Resource.ReferencePath)
	}

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
			Identity: identity,
		}
		return nil
	})
	if err != nil {
		return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()},
			fmt.Errorf("failed to create or update snapshot: %w", err)
	}

	newSnapshot := snapshotCR.DeepCopy()
	newSnapshot.Status.Digest = digest
	newSnapshot.Status.Tag = version
	if err := patchObject(ctx, r.Client, snapshotCR, newSnapshot); err != nil {
		return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()},
			fmt.Errorf("failed to patch snapshot: %w", err)
	}

	log.Info("successfully pushed resource", "resource", obj.Spec.Resource.Name)
	obj.Status.LastAppliedResourceVersion = obj.Spec.Resource.Version

	obj.Status.ObservedGeneration = obj.GetGeneration()

	if err := patchHelper.Patch(ctx, obj); err != nil {
		return ctrl.Result{
			RequeueAfter: obj.GetRequeueAfter(),
		}, fmt.Errorf("failed to patch resource and set snaphost value: %w", err)
	}

	log.Info("successfully reconciled resource", "name", obj.GetName())

	return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, nil
}
