// Copyright 2022.
// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/fluxcd/pkg/runtime/patch"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	deliveryv1alpha1 "github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/pkg/cache"
	"github.com/open-component-model/ocm-controller/pkg/ocm"
)

// ConfigurationReconciler reconciles a Configuration object
type ConfigurationReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	ReconcileInterval time.Duration
	RetryInterval     time.Duration
	Cache             cache.Cache
	OCMClient         ocm.FetchVerifier
}

//+kubebuilder:rbac:groups=delivery.ocm.software,resources=configurations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=delivery.ocm.software,resources=configurations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=delivery.ocm.software,resources=configurations/finalizers,verbs=update

//+kubebuilder:rbac:groups=source.toolkit.fluxcd.io,resources=gitrepositories;buckets;ocirepositories,verbs=get;list;watch

// SetupWithManager sets up the controller with the Manager.
func (r *ConfigurationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	snapshotSourceKey := ".metadata.snapshot.source"
	configKey := ".metadata.config"

	if err := mgr.GetCache().IndexField(context.TODO(), &deliveryv1alpha1.Configuration{}, snapshotSourceKey,
		r.indexBy("Snapshot", "SourceRef")); err != nil {
		return fmt.Errorf("failed setting index fields: %w", err)
	}

	if err := mgr.GetCache().IndexField(context.TODO(), &deliveryv1alpha1.Configuration{}, configKey,
		r.indexBy("ComponentDescriptor", "ConfigRef")); err != nil {
		return fmt.Errorf("failed setting index fields: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&deliveryv1alpha1.Configuration{}).
		Watches(
			&source.Kind{Type: &deliveryv1alpha1.Snapshot{}},
			handler.EnqueueRequestsFromMapFunc(r.requestsForRevisionChangeOf(snapshotSourceKey)),
		).
		Complete(r)
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ConfigurationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithName("configuration-controller")

	obj := &v1alpha1.Configuration{}
	if err := r.Client.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{RequeueAfter: obj.Spec.GetRequeueAfter()}, fmt.Errorf("failed to get configuration object: %w", err)
	}

	log.Info("reconciling configuration")

	return r.reconcile(ctx, obj)
}

func (r *ConfigurationReconciler) reconcile(ctx context.Context, obj *v1alpha1.Configuration) (ctrl.Result, error) {
	// Initialize the patch helper.
	patchHelper, err := patch.NewHelper(obj, r.Client)
	if err != nil {
		return ctrl.Result{RequeueAfter: r.RetryInterval}, err
	}

	mutationLooper := MutationReconcileLooper{
		Scheme:    r.Scheme,
		OCMClient: r.OCMClient,
		Client:    r.Client,
		Cache:     r.Cache,
	}

	digest, err := mutationLooper.ReconcileMutationObject(ctx, obj.Spec, obj)
	if err != nil {
		return ctrl.Result{RequeueAfter: obj.Spec.GetRequeueAfter()}, err
	}
	obj.Status.LatestSnapshotDigest = digest
	if obj.Spec.ConfigRef != nil {
		obj.Status.LatestConfigVersion = fmt.Sprintf("%s:%s", obj.Spec.ConfigRef.Resource.ResourceRef.Name, obj.Spec.ConfigRef.Resource.ResourceRef.Version)
	}
	obj.Status.ObservedGeneration = obj.GetGeneration()

	if err := patchHelper.Patch(ctx, obj); err != nil {
		return ctrl.Result{
			RequeueAfter: obj.Spec.GetRequeueAfter(),
		}, fmt.Errorf("failed to patch resource and set snaphost value: %w", err)
	}

	return ctrl.Result{RequeueAfter: r.RetryInterval}, nil
}

func (r *ConfigurationReconciler) requestsForRevisionChangeOf(indexKey string) func(obj client.Object) []reconcile.Request {
	return func(obj client.Object) []reconcile.Request {
		snap, ok := obj.(*v1alpha1.Snapshot)
		if !ok {
			panic(fmt.Sprintf("expected snapshot but got: %T", obj))
		}

		if snap.Status.LastReconciledDigest == "" {
			return nil
		}

		ctx := context.Background()
		var list v1alpha1.ConfigurationList
		if err := r.List(ctx, &list, client.MatchingFields{
			indexKey: client.ObjectKeyFromObject(obj).String(),
		}); err != nil {
			return nil
		}

		var reqs []reconcile.Request
		for _, d := range list.Items {
			if snap.Status.LastReconciledDigest == d.Status.LatestSnapshotDigest {
				continue
			}
			reqs = append(reqs, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: d.Namespace,
					Name:      d.Name,
				},
			})
		}

		return reqs
	}
}

func (r *ConfigurationReconciler) indexBy(kind, field string) func(o client.Object) []string {
	return func(o client.Object) []string {
		l, ok := o.(*v1alpha1.Configuration)
		if !ok {
			panic(fmt.Sprintf("Expected a Localization, got %T", o))
		}

		switch field {
		case "SourceRef":
			if l.Spec.Source.SourceRef != nil && l.Spec.Source.SourceRef.Kind == kind {
				namespace := l.GetNamespace()
				if l.Spec.Source.SourceRef.Namespace != "" {
					namespace = l.Spec.Source.SourceRef.Namespace
				}
				return []string{fmt.Sprintf("%s/%s", namespace, l.Spec.Source.SourceRef.Name)}
			}
			return []string{fmt.Sprintf("%s/%s", l.Spec.ComponentVersionRef.Namespace, l.Spec.ComponentVersionRef.Name)}
		case "ConfigRef":
			namespace := l.GetNamespace()
			if l.Spec.ComponentVersionRef.Namespace != "" {
				namespace = l.Spec.ComponentVersionRef.Namespace
			}
			return []string{fmt.Sprintf("%s/%s", namespace, strings.ReplaceAll(l.Spec.ComponentVersionRef.Name, "/", "-"))}
		default:
			return nil
		}
	}
}
