// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"errors"
	"fmt"
	"time"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/patch"
	rreconcile "github.com/fluxcd/pkg/runtime/reconcile"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	"golang.org/x/exp/slices"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	kuberecorder "k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/pkg/cache"
	"github.com/open-component-model/ocm-controller/pkg/event"
	"github.com/open-component-model/ocm-controller/pkg/ocm"
	"github.com/open-component-model/ocm-controller/pkg/snapshot"
)

// ConfigurationReconciler reconciles a Configuration object
type ConfigurationReconciler struct {
	client.Client

	DynamicClient dynamic.Interface
	Scheme        *runtime.Scheme
	kuberecorder.EventRecorder
	ReconcileInterval  time.Duration
	RetryInterval      time.Duration
	Cache              cache.Cache
	OCMClient          ocm.Contract
	MutationReconciler MutationReconcileLooper
}

//+kubebuilder:rbac:groups=delivery.ocm.software,resources=configurations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=delivery.ocm.software,resources=configurations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=delivery.ocm.software,resources=configurations/finalizers,verbs=update

//+kubebuilder:rbac:groups=source.toolkit.fluxcd.io,resources=gitrepositories;buckets;ocirepositories,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// SetupWithManager sets up the controller with the Manager.
func (r *ConfigurationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	const (
		sourceKey       = ".metadata.source"
		configKey       = ".metadata.config"
		patchSourceKey  = ".metadata.patchSource"
		valuesSourceKey = ".metadata.fluxValuesSource"
	)

	if err := mgr.GetFieldIndexer().IndexField(context.TODO(), &v1alpha1.Configuration{}, sourceKey, func(rawObj client.Object) []string {
		cfg := rawObj.(*v1alpha1.Configuration)
		var ns = cfg.Spec.SourceRef.Namespace
		if ns == "" {
			ns = cfg.GetNamespace()
		}
		return []string{fmt.Sprintf("%s/%s", ns, cfg.Spec.SourceRef.Name)}
	}); err != nil {
		return fmt.Errorf("failed setting index fields: %w", err)
	}

	if err := mgr.GetFieldIndexer().IndexField(context.TODO(), &v1alpha1.Configuration{}, configKey, func(rawObj client.Object) []string {
		cfg := rawObj.(*v1alpha1.Configuration)
		if cfg.Spec.ConfigRef == nil {
			return nil
		}
		var ns = cfg.Spec.ConfigRef.Namespace
		if ns == "" {
			ns = cfg.GetNamespace()
		}
		return []string{fmt.Sprintf("%s/%s", ns, cfg.Spec.ConfigRef.Name)}
	}); err != nil {
		return fmt.Errorf("failed setting index fields: %w", err)
	}

	if err := mgr.GetFieldIndexer().IndexField(context.TODO(), &v1alpha1.Configuration{}, patchSourceKey, func(rawObj client.Object) []string {
		cfg := rawObj.(*v1alpha1.Configuration)
		if cfg.Spec.PatchStrategicMerge == nil {
			return nil
		}
		var ns = cfg.Spec.PatchStrategicMerge.Source.SourceRef.Namespace
		if ns == "" {
			ns = cfg.GetNamespace()
		}
		return []string{fmt.Sprintf("%s/%s", ns, cfg.Spec.PatchStrategicMerge.Source.SourceRef.Name)}
	}); err != nil {
		return fmt.Errorf("failed setting index fields: %w", err)
	}

	if err := mgr.GetFieldIndexer().IndexField(context.TODO(), &v1alpha1.Configuration{}, valuesSourceKey, func(rawObj client.Object) []string {
		cfg := rawObj.(*v1alpha1.Configuration)
		if cfg.Spec.ValuesFrom == nil {
			return nil
		}
		if cfg.Spec.ValuesFrom.FluxSource == nil {
			return nil
		}
		var ns = cfg.Spec.ValuesFrom.FluxSource.SourceRef.Namespace
		if ns == "" {
			ns = cfg.GetNamespace()
		}
		return []string{fmt.Sprintf("%s/%s", ns, cfg.Spec.ValuesFrom.FluxSource.SourceRef.Name)}
	}); err != nil {
		return fmt.Errorf("failed setting index fields: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Configuration{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(
			&v1alpha1.ComponentVersion{},
			handler.EnqueueRequestsFromMapFunc(r.findObjects(sourceKey, configKey)),
			builder.WithPredicates(ComponentVersionChangedPredicate{}),
		).
		Watches(
			&v1alpha1.Snapshot{},
			handler.EnqueueRequestsFromMapFunc(r.findObjects(sourceKey, configKey)),
			builder.WithPredicates(SnapshotDigestChangedPredicate{}),
		).
		Watches(
			&sourcev1.GitRepository{},
			handler.EnqueueRequestsFromMapFunc(r.findObjectsForGitRepository(patchSourceKey, valuesSourceKey)),
			builder.WithPredicates(SourceRevisionChangePredicate{}),
		).
		Complete(r)
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ConfigurationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	logger := log.FromContext(ctx).WithName("configuration-controller")

	obj := &v1alpha1.Configuration{}
	if err = r.Client.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to get configuration object: %w", err)
	}

	// return early if obj is suspended
	if obj.Spec.Suspend {
		logger.Info("configuration object suspended")
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
				event.New(r.EventRecorder, obj, eventv1.EventSeverityError, err.Error(), nil)
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
			event.New(r.EventRecorder, obj, eventv1.EventSeverityInfo, "Reconciliation succeeded", nil)
		}

		// Set status observed generation option if the object is stalled or ready.
		if conditions.IsStalled(obj) || conditions.IsReady(obj) {
			obj.Status.ObservedGeneration = obj.Generation
			event.New(r.EventRecorder, obj, eventv1.EventSeverityInfo, fmt.Sprintf("Reconciliation finished, next run in %s", obj.GetRequeueAfter()),
				map[string]string{v1alpha1.GroupVersion.Group + "/configuration_digest": obj.Status.LatestSnapshotDigest})
		}

		if perr := patchHelper.Patch(ctx, obj); perr != nil {
			err = errors.Join(err, perr)
		}
	}()

	logger.Info("reconciling configuration")

	// check dependencies are ready
	ready, err := r.checkReadiness(ctx, obj.GetNamespace(), &obj.Spec.SourceRef)
	if err != nil {
		logger.Info("source ref object is not ready with error", "error", err)
		return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, nil
	}
	if !ready {
		logger.Info("source ref object is not ready", "source", obj.Spec.SourceRef.GetNamespacedName())
		return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, nil
	}

	if obj.Spec.ConfigRef != nil {
		ready, err := r.checkReadiness(ctx, obj.GetNamespace(), obj.Spec.ConfigRef)
		if err != nil {
			logger.Info("config ref object is not ready with error", "error", err)
			return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, nil
		}
		if !ready {
			logger.Info("config ref object is not ready", "source", obj.Spec.SourceRef.GetNamespacedName())
			return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, nil
		}
	}

	if obj.Spec.PatchStrategicMerge != nil {
		ready, err := r.checkFluxSourceReadiness(ctx, obj.Spec.PatchStrategicMerge.Source.SourceRef)
		if err != nil {
			logger.Info("source object is not ready with error", "error", err)
			return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, nil
		}

		if !ready {
			ref := obj.Spec.PatchStrategicMerge.Source.SourceRef
			logger.Info("patch git repository object is not ready",
				"gitrepository", (types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}).String())
			return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, nil
		}
	}

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

func (r *ConfigurationReconciler) reconcile(ctx context.Context, obj *v1alpha1.Configuration) (ctrl.Result, error) {
	rreconcile.ProgressiveStatus(false, obj, meta.ProgressingReason, "reconciliation in progress")

	if obj.Generation != obj.Status.ObservedGeneration {
		rreconcile.ProgressiveStatus(false, obj, meta.ProgressingReason,
			"processing object: new generation %d -> %d", obj.Status.ObservedGeneration, obj.Generation)
	}

	err := r.MutationReconciler.ReconcileMutationObject(ctx, obj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, nil
		}

		if errors.Is(err, errTar) {
			err = fmt.Errorf("source resource is not a tar archive: %w", err)
			conditions.MarkFalse(obj, meta.ReadyCondition, v1alpha1.SourceReasonNotATarArchiveReason, err.Error())
			return ctrl.Result{}, err
		}

		err = fmt.Errorf("failed to reconcile mutation object: %w", err)
		conditions.MarkFalse(obj, meta.ReadyCondition, v1alpha1.ReconcileMuationObjectFailedReason, err.Error())
		return ctrl.Result{}, err
	}

	obj.Status.ObservedGeneration = obj.GetGeneration()

	// Remove any stale Ready condition, most likely False, set above. Its value
	// is derived from the overall result of the reconciliation in the deferred
	// bcfgk at the very end.
	conditions.Delete(obj, meta.ReadyCondition)

	return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, nil
}

// The purpose of the findObjects function is to identify whether a given Kubernetes object
// is referenced by a Configuration. This is done by checking whether the object is a ComponentVersion
// or a Snapshot. If it's a ComponentVersion, we look for all Configurations that reference
// it by name. If it's a Snapshot, we first identify its owner and then look for Configurations
// that reference the parent object.
func (r *ConfigurationReconciler) findObjects(sourceKey, configKey string) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		var selectorTerm string

		switch obj.(type) {
		case *v1alpha1.ComponentVersion:
			selectorTerm = client.ObjectKeyFromObject(obj).String()
		case *v1alpha1.Snapshot:
			if len(obj.GetOwnerReferences()) != 1 {
				return []reconcile.Request{}
			}
			selectorTerm = fmt.Sprintf("%s/%s", obj.GetNamespace(), obj.GetOwnerReferences()[0].Name)
		default:
			return []reconcile.Request{}
		}

		sourceRefs := &v1alpha1.ConfigurationList{}
		if err := r.List(context.TODO(), sourceRefs, &client.ListOptions{
			FieldSelector: fields.OneTermEqualSelector(sourceKey, selectorTerm),
		}); err != nil {
			return []reconcile.Request{}
		}

		configRefs := &v1alpha1.ConfigurationList{}
		if err := r.List(context.TODO(), configRefs, &client.ListOptions{
			FieldSelector: fields.OneTermEqualSelector(configKey, selectorTerm),
		}); err != nil {
			return []reconcile.Request{}
		}

		return makeRequestsForConfigurations(append(sourceRefs.Items, configRefs.Items...)...)
	}
}

// this function will enqueue a reconciliation
func (r *ConfigurationReconciler) findObjectsForGitRepository(keys ...string) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		cfgs := &v1alpha1.ConfigurationList{}
		for _, key := range keys {
			result := &v1alpha1.ConfigurationList{}
			if err := r.List(context.TODO(), result, &client.ListOptions{
				FieldSelector: fields.OneTermEqualSelector(key, client.ObjectKeyFromObject(obj).String()),
			}); err != nil {
				return []reconcile.Request{}
			}
			cfgs.Items = append(cfgs.Items, result.Items...)
		}
		return makeRequestsForConfigurations(cfgs.Items...)
	}
}

func (r *ConfigurationReconciler) checkReadiness(ctx context.Context, ns string, obj *v1alpha1.ObjectReference) (bool, error) {
	var ref conditions.Getter
	switch obj.Kind {
	case v1alpha1.ComponentVersionKind:
		if obj.Namespace == "" {
			obj.Namespace = ns
		}
		ref = &v1alpha1.ComponentVersion{}
		if err := r.Get(ctx, obj.GetObjectKey(), ref); err != nil {
			return false, fmt.Errorf("failed to check readiness: %w", err)
		}

	default:
		// if the APIVersion is not set then default to "delivery.ocm.software/v1alpha1"
		if obj.APIVersion == "" {
			obj.APIVersion = v1alpha1.GroupVersion.String()
		}
		// if the Namespace is not set then default to the parent object's namespace
		if obj.Namespace == "" {
			obj.Namespace = ns
		}
		// the dynamic client needs to know the GroupVersionResource for the object it's trying to fetch
		// so construct that and fetch the unstructured object
		gvr := obj.GetGVR()
		src, err := r.DynamicClient.Resource(gvr).Namespace(obj.Namespace).Get(ctx, obj.Name, metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("failed to check readiness: %w", err)
		}

		snapshotName, ok, err := unstructured.NestedString(src.Object, "status", "snapshotName")
		if err != nil {
			return false, fmt.Errorf("failed to check readiness: %w", err)
		}
		if !ok {
			return false, fmt.Errorf("failed to check readiness: %w", err)
		}
		// finally get the snapshot itself
		ref = &v1alpha1.Snapshot{}
		if err := r.Get(ctx, types.NamespacedName{Namespace: obj.Namespace, Name: snapshotName}, ref); err != nil {
			return false, fmt.Errorf("failed to check readiness: %w", err)
		}
	}
	return conditions.IsReady(ref), nil
}

func (r *ConfigurationReconciler) checkFluxSourceReadiness(ctx context.Context, obj meta.NamespacedObjectKindReference) (bool, error) {
	var ref conditions.Getter
	switch obj.Kind {
	case sourcev1.GitRepositoryKind:
		ref = &sourcev1.GitRepository{}
		if err := r.Client.Get(ctx, client.ObjectKey{Namespace: obj.Namespace, Name: obj.Name}, ref); err != nil {
			return false, fmt.Errorf("failed to check flux source readiness: %w", err)
		}
	default:
		return false, fmt.Errorf("kind not compatibile: %s", obj.Kind)
	}
	return conditions.IsReady(ref), nil
}

func makeRequestsForConfigurations(ll ...v1alpha1.Configuration) []reconcile.Request {
	slices.SortFunc(ll, func(a, b v1alpha1.Configuration) int {
		aKey := fmt.Sprintf("%s/%s", a.GetNamespace(), a.GetName())
		bKey := fmt.Sprintf("%s/%s", b.GetNamespace(), b.GetName())

		switch {
		case aKey < bKey:
			return -1
		case aKey == bKey:
			return 0
		}
		return 1
	})

	refs := slices.CompactFunc(ll, func(a, b v1alpha1.Configuration) bool {
		return fmt.Sprintf("%s/%s", a.GetNamespace(), a.GetName()) == fmt.Sprintf("%s/%s", b.GetNamespace(), b.GetName())
	})

	requests := make([]reconcile.Request, len(refs))
	for i, item := range refs {
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
