// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/patch"
	rreconcile "github.com/fluxcd/pkg/runtime/reconcile"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	"github.com/fluxcd/source-controller/api/v1beta2"
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
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/pkg/cache"
	"github.com/open-component-model/ocm-controller/pkg/ocm"
	"github.com/open-component-model/ocm-controller/pkg/snapshot"
)

// LocalizationReconciler reconciles a Localization object
type LocalizationReconciler struct {
	client.Client

	DynamicClient dynamic.Interface
	Scheme        *runtime.Scheme
	kuberecorder.EventRecorder
	ReconcileInterval  time.Duration
	RetryInterval      time.Duration
	OCMClient          ocm.Contract
	Cache              cache.Cache
	MutationReconciler MutationReconcileLooper
}

//+kubebuilder:rbac:groups=delivery.ocm.software,resources=localizations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=delivery.ocm.software,resources=localizations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=delivery.ocm.software,resources=localizations/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// SetupWithManager sets up the controller with the Manager.
func (r *LocalizationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	const (
		sourceKey      = ".metadata.source"
		configKey      = ".metadata.config"
		patchSourceKey = ".metadata.patchSource"
	)

	if err := mgr.GetFieldIndexer().IndexField(context.TODO(), &v1alpha1.Localization{}, sourceKey, func(rawObj client.Object) []string {
		loc := rawObj.(*v1alpha1.Localization)
		ns := loc.Spec.SourceRef.Namespace
		if ns == "" {
			ns = loc.GetNamespace()
		}
		return []string{fmt.Sprintf("%s/%s", ns, loc.Spec.SourceRef.Name)}
	}); err != nil {
		return fmt.Errorf("failed setting index fields: %w", err)
	}

	if err := mgr.GetFieldIndexer().IndexField(context.TODO(), &v1alpha1.Localization{}, configKey, func(rawObj client.Object) []string {
		loc := rawObj.(*v1alpha1.Localization)
		if loc.Spec.ConfigRef == nil {
			return nil
		}

		ns := loc.Spec.ConfigRef.Namespace
		if ns == "" {
			ns = loc.GetNamespace()
		}

		return []string{fmt.Sprintf("%s/%s", ns, loc.Spec.ConfigRef.Name)}
	}); err != nil {
		return fmt.Errorf("failed setting index fields: %w", err)
	}

	if err := mgr.GetFieldIndexer().IndexField(context.TODO(), &v1alpha1.Localization{}, patchSourceKey, func(rawObj client.Object) []string {
		loc := rawObj.(*v1alpha1.Localization)
		if loc.Spec.PatchStrategicMerge == nil {
			return nil
		}
		ns := loc.Spec.PatchStrategicMerge.Source.SourceRef.Namespace
		if ns == "" {
			ns = loc.GetNamespace()
		}
		return []string{fmt.Sprintf("%s/%s", ns, loc.Spec.PatchStrategicMerge.Source.SourceRef.Name)}
	}); err != nil {
		return fmt.Errorf("failed setting index fields: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Localization{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(
			&source.Kind{Type: &v1alpha1.ComponentVersion{}},
			handler.EnqueueRequestsFromMapFunc(r.findObjects(sourceKey, configKey)),
			builder.WithPredicates(ComponentVersionChangedPredicate{}),
		).
		Watches(
			&source.Kind{Type: &v1alpha1.Snapshot{}},
			handler.EnqueueRequestsFromMapFunc(r.findObjects(sourceKey, configKey)),
			builder.WithPredicates(SnapshotDigestChangedPredicate{}),
		).
		Watches(
			&source.Kind{Type: &v1beta2.GitRepository{}},
			handler.EnqueueRequestsFromMapFunc(r.findObjectsForGitRepository(patchSourceKey)),
			builder.WithPredicates(SourceRevisionChangePredicate{}),
		).
		Complete(r)
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *LocalizationReconciler) Reconcile(
	ctx context.Context,
	req ctrl.Request,
) (result ctrl.Result, err error) {
	obj := &v1alpha1.Localization{}
	if err = r.Client.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to get localization object: %w", err)
	}

	// return early if obj is suspended
	if obj.Spec.Suspend {
		return result, nil
	}

	patchHelper := patch.NewSerialPatcher(obj, r.Client)

	// Always attempt to patch the object and status after each reconciliation.
	defer func() {
		if derr := updateStatus(ctx, patchHelper, obj, r.EventRecorder, obj.GetRequeueAfter()); derr != nil {
			err = errors.Join(err, derr)
		}
	}()

	// Starts the progression by setting ReconcilingCondition.
	// This will be checked in defer.
	// Should only be deleted on a success.
	rreconcile.ProgressiveStatus(false, obj, meta.ProgressingReason, "reconciliation in progress for localization: %s", obj.Name)

	// check dependencies are ready
	ready, err := r.checkReadiness(ctx, obj.GetNamespace(), &obj.Spec.SourceRef)
	if err != nil {
		MarkNotReady(r.EventRecorder, obj, "SourceRefNotReadyWithError", err.Error())

		return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, nil
	}

	if !ready {
		MarkNotReady(
			r.EventRecorder,
			obj,
			"SourceRefNotReady",
			fmt.Sprintf("source ref not yet ready: %s", obj.Spec.SourceRef.Name),
		)

		return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, nil
	}

	if obj.Spec.ConfigRef != nil {
		ready, err := r.checkReadiness(ctx, obj.GetNamespace(), obj.Spec.ConfigRef)
		if err != nil {
			MarkNotReady(
				r.EventRecorder,
				obj,
				"ConfigRefNotReadyWithError",
				fmt.Sprintf("config ref not yet ready with error: %s: %s", obj.Spec.ConfigRef.Name, err),
			)

			return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, nil
		}
		if !ready {
			MarkNotReady(
				r.EventRecorder,
				obj,
				"ConfigRefNotReady",
				fmt.Sprintf("config ref not yet ready: %s", obj.Spec.ConfigRef.Name),
			)

			return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, nil
		}
	}

	if obj.Spec.PatchStrategicMerge != nil {
		ready, err := r.checkFluxSourceReadiness(ctx, obj.Spec.PatchStrategicMerge.Source.SourceRef)
		if err != nil {
			MarkNotReady(
				r.EventRecorder,
				obj,
				"PatchStrategicMergeSourceRefNotReadyWithError",
				fmt.Sprintf("patch strategic merge source ref not yet ready with error: %s: %s", obj.Spec.PatchStrategicMerge.Source.SourceRef.Name, err),
			)

			return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, nil
		}

		if !ready {
			MarkNotReady(
				r.EventRecorder,
				obj,
				"PatchStrategicMergeSourceRefNotReady",
				fmt.Sprintf("patch strategic merge source ref not yet ready: %s", obj.Spec.PatchStrategicMerge.Source.SourceRef.Name),
			)

			return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, nil
		}
	}

	// if the snapshot name has not been generated then
	// generate, patch the status and requeue
	if obj.GetSnapshotName() == "" {
		name, err := snapshot.GenerateSnapshotName(obj.GetName())
		if err != nil {
			err = fmt.Errorf("failed to generate snapshot name for: %s: %s", obj.GetName(), err)
			MarkNotReady(r.EventRecorder, obj, v1alpha1.NameGenerationFailedReason, err.Error())

			return ctrl.Result{}, err
		}

		obj.Status.SnapshotName = name
		return ctrl.Result{Requeue: true}, nil
	}

	return r.reconcile(ctx, obj)
}

func (r *LocalizationReconciler) reconcile(
	ctx context.Context,
	obj *v1alpha1.Localization,
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

	if err := r.MutationReconciler.ReconcileMutationObject(ctx, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, nil
		}

		if errors.Is(err, errTar) {
			err = fmt.Errorf("source resource is not a tar archive: %w", err)
			MarkNotReady(r.EventRecorder, obj, v1alpha1.SourceReasonNotATarArchiveReason, err.Error())

			return ctrl.Result{}, err
		}

		err = fmt.Errorf("failed to reconcile mutation object: %w", err)
		MarkNotReady(r.EventRecorder, obj, v1alpha1.ReconcileMutationObjectFailedReason, err.Error())

		return ctrl.Result{}, err
	}

	conditions.MarkTrue(obj,
		meta.ReadyCondition,
		meta.SucceededReason,
		"Reconciliation success")

	conditions.Delete(obj, meta.ReconcilingCondition)

	return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, nil
}

// The purpose of the findObjects function is to identify whether a given Kubernetes object
// is referenced by a Localization. This is done by checking whether the object is a ComponentVersion
// or a Snapshot. If it's a ComponentVersion, we look for all Configurations that reference
// it by name. If it's a Snapshot, we first identify its owner and then look for Localization
// that reference the parent object.
func (r *LocalizationReconciler) findObjects(sourceKey, configKey string) handler.MapFunc {
	return func(obj client.Object) []reconcile.Request {
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

		sourceRefs := &v1alpha1.LocalizationList{}
		if err := r.List(context.TODO(), sourceRefs, &client.ListOptions{
			FieldSelector: fields.OneTermEqualSelector(sourceKey, selectorTerm),
		}); err != nil {
			return []reconcile.Request{}
		}

		configRefs := &v1alpha1.LocalizationList{}
		if err := r.List(context.TODO(), configRefs, &client.ListOptions{
			FieldSelector: fields.OneTermEqualSelector(configKey, selectorTerm),
		}); err != nil {
			return []reconcile.Request{}
		}
		return makeRequestsForLocalizations(append(sourceRefs.Items, configRefs.Items...)...)
	}
}

// this function will enqueue a reconciliation for any component version which is referenced
// in the .spec.sourceRef or spec.configRef field of a Localization
func (r *LocalizationReconciler) findObjectsForGitRepository(key string) handler.MapFunc {
	return func(obj client.Object) []reconcile.Request {
		patchRefs := &v1alpha1.LocalizationList{}
		if err := r.List(context.TODO(), patchRefs, &client.ListOptions{
			FieldSelector: fields.OneTermEqualSelector(key, client.ObjectKeyFromObject(obj).String()),
		}); err != nil {
			return []reconcile.Request{}
		}
		return makeRequestsForLocalizations(patchRefs.Items...)
	}
}

func (r *LocalizationReconciler) checkReadiness(
	ctx context.Context,
	ns string,
	obj *v1alpha1.ObjectReference,
) (bool, error) {
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
		src, err := r.DynamicClient.Resource(gvr).
			Namespace(obj.Namespace).
			Get(ctx, obj.Name, metav1.GetOptions{})
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

func (r *LocalizationReconciler) checkFluxSourceReadiness(
	ctx context.Context,
	obj meta.NamespacedObjectKindReference,
) (bool, error) {
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

func makeRequestsForLocalizations(ll ...v1alpha1.Localization) []reconcile.Request {
	slices.SortFunc(ll, func(a, b v1alpha1.Localization) bool {
		aKey := fmt.Sprintf("%s/%s", a.GetNamespace(), a.GetName())
		bKey := fmt.Sprintf("%s/%s", b.GetNamespace(), b.GetName())
		return aKey < bKey
	})

	refs := slices.CompactFunc(ll, func(a, b v1alpha1.Localization) bool {
		return fmt.Sprintf(
			"%s/%s",
			a.GetNamespace(),
			a.GetName(),
		) == fmt.Sprintf(
			"%s/%s",
			b.GetNamespace(),
			b.GetName(),
		)
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
