// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"errors"
	"fmt"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/patch"
	rreconcile "github.com/fluxcd/pkg/runtime/reconcile"
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
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/pkg/cache"
	"github.com/open-component-model/ocm-controller/pkg/component"
	"github.com/open-component-model/ocm-controller/pkg/event"
	"github.com/open-component-model/ocm-controller/pkg/ocm"
	"github.com/open-component-model/ocm-controller/pkg/snapshot"
	ocmmetav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
)

// ResourceReconciler reconciles a Resource object
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
		res := rawObj.(*v1alpha1.Resource)
		return []string{res.Spec.SourceRef.Name}

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
func (r *ResourceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	logger := log.FromContext(ctx).WithName("resource-controller")

	obj := &v1alpha1.Resource{}
	if err = r.Client.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to get resource object: %w", err)
	}

	if obj.Spec.Suspend {
		logger.Info("resource object suspended")
		return result, nil
	}

	var patchHelper *patch.Helper
	patchHelper, err = patch.NewHelper(obj, r.Client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create patch helper: %w", err)
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
		if result.RequeueAfter == obj.GetRequeueAfter() && !result.Requeue && err == nil {
			// Remove the reconciling condition if it's set.
			conditions.Delete(obj, meta.ReconcilingCondition)

			// Set the return err as the ready failure message is the resource is not ready, but also not reconciling or stalled.
			if ready := conditions.Get(obj, meta.ReadyCondition); ready != nil && ready.Status == metav1.ConditionFalse && !conditions.IsStalled(obj) {
				err = errors.New(conditions.GetMessage(obj, meta.ReadyCondition))
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
			err == nil && result.RequeueAfter == obj.GetRequeueAfter() {
			conditions.MarkTrue(obj, meta.ReadyCondition, meta.SucceededReason, "Reconciliation success")
			event.New(r.EventRecorder, obj, eventv1.EventSeverityInfo, "Reconciliation success", nil)
		}

		// Set status observed generation option if the object is stalled or ready.
		if conditions.IsStalled(obj) || conditions.IsReady(obj) {
			obj.Status.ObservedGeneration = obj.Generation
			event.New(r.EventRecorder, obj, eventv1.EventSeverityInfo, fmt.Sprintf("Reconciliation finished, next run in %s", obj.GetRequeueAfter()),
				map[string]string{v1alpha1.GroupVersion.Group + "/resource_version": obj.Status.LastAppliedResourceVersion})
		}

		if perr := patchHelper.Patch(ctx, obj); perr != nil {
			err = errors.Join(err, perr)
		}
	}()

	// if the snapshot name has not been generated then
	// generate, patch the status and requeue
	if obj.GetSnapshotName() == "" {
		name, err := snapshot.GenerateSnapshotName(obj.GetName())
		if err != nil {
			return ctrl.Result{}, err
		}
		obj.Status.SnapshotName = name
		return ctrl.Result{Requeue: true}, nil
	}

	return r.reconcile(ctx, obj)
}

func (r *ResourceReconciler) reconcile(ctx context.Context, obj *v1alpha1.Resource) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithName("resource-controller")

	rreconcile.ProgressiveStatus(false, obj, meta.ProgressingReason, "reconciliation in progress")

	if obj.Generation != obj.Status.ObservedGeneration {
		rreconcile.ProgressiveStatus(false, obj, meta.ProgressingReason,
			"processing object: new generation %d -> %d", obj.Status.ObservedGeneration, obj.Generation)
	}

	if obj.Spec.SourceRef.Namespace == "" {
		obj.Spec.SourceRef.Namespace = obj.GetNamespace()
	}

	conditions.Delete(obj, meta.StalledCondition)

	var componentVersion v1alpha1.ComponentVersion
	if err := r.Get(ctx, obj.Spec.SourceRef.GetObjectKey(), &componentVersion); err != nil {
		err = fmt.Errorf("failed to get component version: %w", err)
		conditions.MarkFalse(obj, meta.ReadyCondition, v1alpha1.GetResourceFailedReason, err.Error())
		event.New(r.EventRecorder, obj, eventv1.EventSeverityError, err.Error(), nil)
		return ctrl.Result{}, err
	}

	octx, err := r.OCMClient.CreateAuthenticatedOCMContext(ctx, &componentVersion)
	if err != nil {
		err = fmt.Errorf("failed to create authenticated client: %w", err)
		conditions.MarkFalse(obj, meta.ReadyCondition, v1alpha1.AuthenticatedContextCreationFailedReason, err.Error())
	}

	reader, digest, err := r.OCMClient.GetResource(ctx, octx, &componentVersion, obj.Spec.SourceRef.ResourceRef)
	if err != nil {
		err = fmt.Errorf("failed to get resource: %w", err)
		conditions.MarkFalse(obj, meta.ReadyCondition, v1alpha1.GetResourceFailedReason, err.Error())
		event.New(r.EventRecorder, obj, eventv1.EventSeverityError, err.Error(), nil)
		return ctrl.Result{}, err
	}
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
		conditions.MarkFalse(obj, meta.ReadyCondition, v1alpha1.GetComponentDescriptorFailedReason, err.Error())
		event.New(r.EventRecorder, obj, eventv1.EventSeverityError, err.Error(), nil)
		return ctrl.Result{}, err
	}

	if componentDescriptor == nil {
		err := fmt.Errorf("couldn't find component descriptor for reference '%s' or any root components", obj.GetReferencePath())
		conditions.MarkFalse(obj, meta.ReadyCondition, v1alpha1.ComponentDescriptorNotFoundReason, err.Error())
		// Mark stalled because we can't do anything until the component descriptor is available. Likely requires some sort of manual intervention.
		conditions.MarkStalled(obj, v1alpha1.ComponentDescriptorNotFoundReason, err.Error())
		event.New(r.EventRecorder, obj, eventv1.EventSeverityError, err.Error(), nil)

		return ctrl.Result{}, nil
	}

	conditions.Delete(obj, meta.StalledCondition)

	identity := ocmmetav1.Identity{
		v1alpha1.ComponentNameKey:    componentDescriptor.Name,
		v1alpha1.ComponentVersionKey: componentDescriptor.Spec.Version,
		v1alpha1.ResourceNameKey:     obj.Spec.SourceRef.ResourceRef.Name,
		v1alpha1.ResourceVersionKey:  version,
	}
	for k, v := range obj.Spec.SourceRef.ResourceRef.ExtraIdentity {
		identity[k] = v
	}

	if obj.GetSnapshotName() == "" {
		return ctrl.Result{}, fmt.Errorf("snapshot name should not be empty")
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
		conditions.MarkFalse(obj, meta.ReadyCondition, v1alpha1.CreateOrUpdateSnapshotFailedReason, err.Error())
		event.New(r.EventRecorder, obj, eventv1.EventSeverityError, err.Error(), nil)
		return ctrl.Result{}, err
	}

	logger.Info("successfully pushed snapshot for resource", "resource", obj.Spec.SourceRef.Name)

	obj.Status.LastAppliedResourceVersion = obj.Spec.SourceRef.GetVersion()
	obj.Status.ObservedGeneration = obj.GetGeneration()
	obj.Status.LastAppliedComponentVersion = componentVersion.Status.ReconciledVersion

	logger.Info("successfully reconciled resource", "name", obj.GetName())

	// Remove any stale Ready condition, most likely False, set above. Its value
	// is derived from the overall result of the reconciliation in the deferred
	// block at the very end.
	conditions.Delete(obj, meta.ReadyCondition)

	return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, nil
}

// this function will enqueue a reconciliation for any snapshot which is referenced
// in the .spec.sourceRef or spec.configRef field of a Localization
func (r *ResourceReconciler) findObjects(key string) func(client.Object) []reconcile.Request {
	return func(obj client.Object) []reconcile.Request {
		resources := &v1alpha1.ResourceList{}
		if err := r.List(context.TODO(), resources, &client.ListOptions{
			FieldSelector: fields.OneTermEqualSelector(key, obj.GetName()),
			Namespace:     obj.GetNamespace(),
		}); err != nil {
			return []reconcile.Request{}
		}

		requests := make([]reconcile.Request, len(resources.Items))
		for i, item := range resources.Items {
			// if the observedgeneration is -1
			// then the object has not been initialised yet
			// so we should not trigger a reconcilation for sourceRef/configRefs
			if item.Status.ObservedGeneration != -1 {
				requests[i] = reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      item.GetName(),
						Namespace: item.GetNamespace(),
					},
				}
			}
		}

		return requests
	}
}
