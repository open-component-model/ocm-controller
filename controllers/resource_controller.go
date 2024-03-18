// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"errors"
	"fmt"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/patch"
	rreconcile "github.com/fluxcd/pkg/runtime/reconcile"
	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/pkg/cache"
	"github.com/open-component-model/ocm-controller/pkg/component"
	"github.com/open-component-model/ocm-controller/pkg/metrics"
	"github.com/open-component-model/ocm-controller/pkg/ocm"
	"github.com/open-component-model/ocm-controller/pkg/snapshot"
	"github.com/open-component-model/ocm-controller/pkg/status"
	ocmmetav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	mh "github.com/open-component-model/pkg/metrics"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kuberecorder "k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// ResourceReconciler reconciles a Resource object.
type ResourceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	kuberecorder.EventRecorder
	OCMClient ocm.Contract
	Cache     cache.Cache
}

// +kubebuilder:rbac:groups=delivery.ocm.software,resources=resources,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=delivery.ocm.software,resources=resources/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=delivery.ocm.software,resources=resources/finalizers,verbs=update

// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// SetupWithManager sets up the controller with the Manager.
func (r *ResourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	const (
		resourceKey = ".metadata.resource"
	)

	if err := mgr.GetFieldIndexer().IndexField(context.TODO(), &v1alpha1.Resource{}, resourceKey, func(rawObj client.Object) []string {
		res, ok := rawObj.(*v1alpha1.Resource)
		if !ok {
			return nil
		}

		ns := res.Spec.SourceRef.Namespace
		if ns == "" {
			ns = res.GetNamespace()
		}

		return []string{fmt.Sprintf("%s/%s", ns, res.Spec.SourceRef.Name)}
	}); err != nil {
		return fmt.Errorf("failed setting index fields: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Resource{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(
			&source.Kind{Type: &v1alpha1.ComponentVersion{}},
			handler.EnqueueRequestsFromMapFunc(r.findObjects(resourceKey)),
			builder.WithPredicates(ComponentVersionChangedPredicate{}),
		).
		Complete(r)
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ResourceReconciler) Reconcile(
	ctx context.Context,
	req ctrl.Request,
) (result ctrl.Result, err error) {
	obj := &v1alpha1.Resource{}
	if err = r.Client.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to get resource object: %w", err)
	}

	if obj.Spec.Suspend {
		return result, nil
	}

	patchHelper := patch.NewSerialPatcher(obj, r.Client)

	// Always attempt to patch the object and status after each reconciliation.
	defer func() {
		if derr := status.UpdateStatus(ctx, patchHelper, obj, r.EventRecorder, obj.GetRequeueAfter()); derr != nil {
			err = errors.Join(err, derr)
		}

		if err != nil {
			metrics.ResourceReconcileFailed.WithLabelValues(obj.Name).Inc()
		}
	}()

	// Starts the progression by setting ReconcilingCondition.
	// This will be checked in defer.
	// Should only be deleted on a success.
	rreconcile.ProgressiveStatus(false, obj, meta.ProgressingReason, "reconciliation in progress for resource: %s", obj.Name)

	// if the snapshot name has not been generated then
	// generate, patch the status and requeue
	if obj.GetSnapshotName() == "" {
		name, err := snapshot.GenerateSnapshotName(obj.GetName())
		if err != nil {
			err = fmt.Errorf("failed to generate snapshot name for: %s: %w", obj.GetName(), err)
			status.MarkNotReady(r.EventRecorder, obj, v1alpha1.NameGenerationFailedReason, err.Error())

			return ctrl.Result{}, err
		}

		obj.Status.SnapshotName = name

		return ctrl.Result{Requeue: true}, nil
	}

	return r.reconcile(ctx, obj)
}

func (r *ResourceReconciler) reconcile(
	ctx context.Context,
	obj *v1alpha1.Resource,
) (ctrl.Result, error) {
	if obj.Generation != obj.Status.ObservedGeneration {
		rreconcile.ProgressiveStatus(
			false,
			obj,
			meta.ProgressingReason,
			"processing object: new generation %d -> %d",
			obj.Status.ObservedGeneration,
			obj.Generation,
		)
	}

	if obj.Spec.SourceRef.Namespace == "" {
		obj.Spec.SourceRef.Namespace = obj.GetNamespace()
	}

	if obj.GetSnapshotName() == "" {
		err := errors.New("snapshot name should not be empty")
		status.MarkNotReady(r.EventRecorder, obj, v1alpha1.SnapshotNameEmptyReason, err.Error())

		return ctrl.Result{}, err
	}

	var componentVersion v1alpha1.ComponentVersion
	if err := r.Get(ctx, obj.Spec.SourceRef.GetObjectKey(), &componentVersion); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, nil
		}

		err = fmt.Errorf("failed to get component version: %w", err)
		status.MarkNotReady(r.EventRecorder, obj, v1alpha1.ComponentVersionNotFoundReason, err.Error())

		return ctrl.Result{}, err
	}

	if !conditions.IsReady(&componentVersion) {
		status.MarkNotReady(r.EventRecorder, obj, v1alpha1.ComponentVersionNotReadyReason, "component version not ready yet")

		return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, nil
	}

	rreconcile.ProgressiveStatus(false, obj, meta.ProgressingReason, "component version %s ready, processing ocm resource", componentVersion.Name)

	octx, err := r.OCMClient.CreateAuthenticatedOCMContext(ctx, &componentVersion)
	if err != nil {
		err = fmt.Errorf("failed to create authenticated client: %w", err)
		status.MarkAsStalled(r.EventRecorder, obj, v1alpha1.AuthenticatedContextCreationFailedReason, err.Error())

		return ctrl.Result{}, nil
	}

	reader, digest, size, err := r.OCMClient.GetResource(ctx, octx, &componentVersion, obj.Spec.SourceRef.ResourceRef)
	if err != nil {
		err = fmt.Errorf("failed to get resource: %w", err)
		status.MarkNotReady(r.EventRecorder, obj, v1alpha1.GetResourceFailedReason, err.Error())

		return ctrl.Result{}, err
	}
	// The reader is unused here, but we should still close it, so it's not left over.
	defer reader.Close()

	version := "latest"
	if obj.Spec.SourceRef.GetVersion() != "" {
		version = obj.Spec.SourceRef.GetVersion()
	}

	// This is important because THIS is the actual component for our resource. If we used ComponentVersion in the
	// below identity, that would be the top-level component instead of the component that this resource belongs to.
	componentDescriptor, err := component.GetComponentDescriptor(ctx, r.Client, obj.GetReferencePath(), componentVersion.Status.ComponentDescriptor)
	if err != nil {
		err = fmt.Errorf("failed to get component descriptor for resource: %w", err)
		status.MarkNotReady(r.EventRecorder, obj, v1alpha1.GetComponentDescriptorFailedReason, err.Error())

		return ctrl.Result{}, err
	}

	if componentDescriptor == nil {
		err := fmt.Errorf(
			"couldn't find component descriptor for reference '%s' or any root components",
			obj.GetReferencePath(),
		)
		status.MarkNotReady(r.EventRecorder, obj, v1alpha1.ComponentDescriptorNotFoundReason, err.Error())

		return ctrl.Result{}, err
	}

	rreconcile.ProgressiveStatus(false, obj, meta.ProgressingReason, "resource retrieve, constructing snapshot with name %s", obj.GetSnapshotName())

	identity := ocmmetav1.Identity{
		v1alpha1.ComponentNameKey:    componentDescriptor.Name,
		v1alpha1.ComponentVersionKey: componentDescriptor.Spec.Version,
		v1alpha1.ResourceNameKey:     obj.Spec.SourceRef.ResourceRef.Name,
		v1alpha1.ResourceVersionKey:  version,
	}
	for k, v := range obj.Spec.SourceRef.ResourceRef.ExtraIdentity {
		identity[k] = v
	}

	snapshotCR := &v1alpha1.Snapshot{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: obj.GetNamespace(),
			Name:      obj.GetSnapshotName(),
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
			Digest:   digest,
			Tag:      version,
		}

		return nil
	})
	if err != nil {
		err = fmt.Errorf("failed to create or update snapshot: %w", err)
		status.MarkNotReady(r.EventRecorder, obj, v1alpha1.CreateOrUpdateSnapshotFailedReason, err.Error())

		return ctrl.Result{}, err
	}

	obj.Status.LastAppliedResourceVersion = obj.Spec.SourceRef.GetVersion()
	obj.Status.LastAppliedComponentVersion = componentVersion.Status.ReconciledVersion

	metrics.SnapshotNumberOfBytesReconciled.WithLabelValues(obj.GetSnapshotName(), digest, componentVersion.Name).Set(float64(size))
	metrics.ResourceReconcileSuccess.WithLabelValues(obj.Name).Inc()

	if product := IsProductOwned(obj); product != "" {
		metrics.MPASResourceReconciledStatus.WithLabelValues(product, mh.MPASStatusSuccess).Inc()
	}

	status.MarkReady(r.EventRecorder, obj, fmt.Sprintf("Applied version: %s", obj.Status.LastAppliedComponentVersion))

	return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, nil
}

// this function will enqueue a reconciliation for any snapshot which is referenced
// in the .spec.sourceRef or spec.configRef field of a Localization.
func (r *ResourceReconciler) findObjects(key string) handler.MapFunc {
	return func(obj client.Object) []reconcile.Request {
		resources := &v1alpha1.ResourceList{}
		if err := r.List(context.TODO(), resources, &client.ListOptions{
			FieldSelector: fields.OneTermEqualSelector(key, client.ObjectKeyFromObject(obj).String()),
		}); err != nil {
			return []reconcile.Request{}
		}

		requests := make([]reconcile.Request, len(resources.Items))
		for i, item := range resources.Items {
			requests[i] = reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      item.GetName(),
					Namespace: item.GetNamespace(),
				},
			}
		}

		return requests
	}
}
