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

	"github.com/fluxcd/pkg/runtime/patch"
	"github.com/fluxcd/source-controller/api/v1beta2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kuberecorder "k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	rreconcile "github.com/fluxcd/pkg/runtime/reconcile"
	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/pkg/cache"
	"github.com/open-component-model/ocm-controller/pkg/event"
	"github.com/open-component-model/ocm-controller/pkg/ocm"
)

// LocalizationReconciler reconciles a Localization object
type LocalizationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	kuberecorder.EventRecorder
	ReconcileInterval time.Duration
	RetryInterval     time.Duration
	OCMClient         ocm.Contract
	Cache             cache.Cache
}

//+kubebuilder:rbac:groups=delivery.ocm.software,resources=localizations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=delivery.ocm.software,resources=localizations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=delivery.ocm.software,resources=localizations/finalizers,verbs=update

// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// SetupWithManager sets up the controller with the Manager.
func (r *LocalizationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	snapshotSourceKey := ".metadata.snapshot.source"
	configKey := ".metadata.config"

	if err := mgr.GetCache().IndexField(context.TODO(), &v1alpha1.Localization{}, snapshotSourceKey,
		r.indexBy("Snapshot", "SourceRef")); err != nil {
		return fmt.Errorf("failed setting index fields: %w", err)
	}

	if err := mgr.GetCache().IndexField(context.TODO(), &v1alpha1.Localization{}, configKey,
		r.indexBy("ComponentDescriptor", "ConfigRef")); err != nil {
		return fmt.Errorf("failed setting index fields: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Localization{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(
			&source.Kind{Type: &v1alpha1.Snapshot{}},
			handler.EnqueueRequestsFromMapFunc(r.requestsForRevisionChangeOf(snapshotSourceKey)),
		).
		Complete(r)
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *LocalizationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	log := log.FromContext(ctx).WithName("localization-controller")

	obj := &v1alpha1.Localization{}
	if err = r.Client.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to get localization object: %w", err)
	}

	if obj.Spec.Suspend {
		log.Info("localization object suspended")

		return
	}

	cv := types.NamespacedName{
		Name:      obj.Spec.ComponentVersionRef.Name,
		Namespace: obj.Spec.ComponentVersionRef.Namespace,
	}

	componentVersion := &v1alpha1.ComponentVersion{}
	if err = r.Get(ctx, cv, componentVersion); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get component object: %w", err)
	}

	var run bool
	run, err = r.shouldReconcile(ctx, componentVersion, obj)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to check if controller should reconcile: %w", err)
	}

	if !run {
		log.Info("no reconciling needed, requeuing", "component-version", componentVersion.Status.ReconciledVersion)
		return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, nil
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
				map[string]string{v1alpha1.GroupVersion.Group + "/localization_digest": obj.Status.LatestSnapshotDigest})
		}

		if perr := patchHelper.Patch(ctx, obj); perr != nil {
			err = errors.Join(err, perr)
		}
	}()

	log.Info("reconciling localization")

	return r.reconcile(ctx, componentVersion, obj)
}

// shouldReconcile deals with the following cases:
// - if the last applied component version does NOT match the ReconciledVersion reconciliation should _PROCEED_
// If the component version are the same, we deal with two further cases:
//   - the snapshot that the reconciliation would produce is not found yet; the reconciliation should _PROCEED_
//   - the snapshot IS found, but it's not Ready yet ( this could be caused by transient error ) and needs a potential
//     update; the reconciliation should _PROCEED_
//
// If neither of these cases match, the reconciliation should _STOP_ and requeue the object.
// For mutating objects, we include two other checks. These objects can have a Source object defined as a Snapshot.
// We also want to track the condition of those source objects. If the last seen digest or tag of those Snapshots
// changed, we should reconcile and see if there is anything new we need to apply.
func (r *LocalizationReconciler) shouldReconcile(ctx context.Context, cv *v1alpha1.ComponentVersion, obj *v1alpha1.Localization) (bool, error) {
	// If there is a mismatch between the observed generation of a component version, we trigger
	// a reconcile. There is either a new version available or a dependent component version
	// finished its reconcile process.
	if obj.Status.LastAppliedComponentVersion != cv.Status.ReconciledVersion {
		return true, nil
	}

	// if source is a snapshot, check on its status
	if obj.Spec.Source.SourceRef != nil {
		d, err := r.getDigestFromSource(ctx, obj.Spec.Source.SourceRef)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}

		if obj.Status.LastAppliedSourceDigest != d {
			return true, nil
		}
	}

	if obj.Spec.ConfigRef != nil && obj.Spec.ConfigRef.Resource.SourceRef != nil {
		d, err := r.getDigestFromSource(ctx, obj.Spec.ConfigRef.Resource.SourceRef)
		if err != nil {
			return false, err
		}

		if obj.Status.LastAppliedConfigSourceDigest != d {
			return true, nil
		}
	}

	if obj.Spec.PatchStrategicMerge != nil {
		d, err := r.getDigestFromSource(ctx, &meta.NamespacedObjectKindReference{
			Kind:      obj.Spec.PatchStrategicMerge.Source.SourceRef.Kind,
			Name:      obj.Spec.PatchStrategicMerge.Source.SourceRef.Name,
			Namespace: obj.Spec.PatchStrategicMerge.Source.SourceRef.Namespace,
		})
		if err != nil {
			return false, err
		}

		if obj.Status.LastAppliedPatchMergeSourceDigest != d {
			return true, nil
		}
	}

	// If there is no mismatch, we check if we are already done with our snapshot.
	snapshot := &v1alpha1.Snapshot{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      obj.Spec.SnapshotTemplate.Name,
		Namespace: obj.Namespace,
	}, snapshot); err != nil {
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, fmt.Errorf("failed to get snapshot for localization object: %w", err)
	}

	// If there is no ready condition, we should return true to trigger a reconcile loop.
	return conditions.IsFalse(snapshot, meta.ReadyCondition), nil
}

func (r *LocalizationReconciler) reconcile(ctx context.Context, cv *v1alpha1.ComponentVersion, obj *v1alpha1.Localization) (ctrl.Result, error) {
	rreconcile.ProgressiveStatus(false, obj, meta.ProgressingReason, "reconciliation in progress")

	if obj.Generation != obj.Status.ObservedGeneration {
		rreconcile.ProgressiveStatus(false, obj, meta.ProgressingReason,
			"processing object: new generation %d -> %d", obj.Status.ObservedGeneration, obj.Generation)
	}

	mutationLooper := MutationReconcileLooper{
		Scheme:    r.Scheme,
		OCMClient: r.OCMClient,
		Client:    r.Client,
		Cache:     r.Cache,
	}

	digest, err := mutationLooper.ReconcileMutationObject(ctx, cv, obj.Spec, obj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, nil
		}

		if errors.Is(err, tarError) {
			err = fmt.Errorf("source resource is not a tar archive: %w", err)
			conditions.MarkFalse(obj, meta.ReadyCondition, v1alpha1.SourceReasonNotATarArchiveReason, err.Error())
			return ctrl.Result{}, err
		}

		err = fmt.Errorf("failed to reconcile mutation object: %w", err)
		conditions.MarkFalse(obj, meta.ReadyCondition, v1alpha1.ReconcileMuationObjectFailedReason, err.Error())
		event.New(r.EventRecorder, obj, eventv1.EventSeverityError, err.Error(), nil)

		return ctrl.Result{}, err
	}

	obj.Status.LatestSnapshotDigest = digest
	obj.Status.LatestConfigVersion = fmt.Sprintf("%s:%s", obj.Spec.ConfigRef.Resource.ResourceRef.Name, obj.Spec.ConfigRef.Resource.ResourceRef.Version)
	obj.Status.ObservedGeneration = obj.GetGeneration()
	obj.Status.LastAppliedComponentVersion = cv.Status.ReconciledVersion

	if obj.Spec.Source.SourceRef != nil {
		d, err := r.getDigestFromSource(ctx, obj.Spec.Source.SourceRef)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to get digest from source: %w", err)
		}
		obj.Status.LastAppliedSourceDigest = d
	}
	if obj.Spec.ConfigRef != nil && obj.Spec.ConfigRef.Resource.SourceRef != nil {
		d, err := r.getDigestFromSource(ctx, obj.Spec.ConfigRef.Resource.SourceRef)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to get digest from config source: %w", err)
		}
		obj.Status.LastAppliedConfigSourceDigest = d
	}
	if obj.Spec.PatchStrategicMerge != nil {
		d, err := r.getDigestFromSource(ctx, &meta.NamespacedObjectKindReference{
			Kind:      obj.Spec.PatchStrategicMerge.Source.SourceRef.Kind,
			Name:      obj.Spec.PatchStrategicMerge.Source.SourceRef.Name,
			Namespace: obj.Spec.PatchStrategicMerge.Source.SourceRef.Namespace,
		})
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to get digest from patch merge source: %w", err)
		}
		obj.Status.LastAppliedPatchMergeSourceDigest = d
	}

	// Remove any stale Ready condition, most likely False, set above. Its value
	// is derived from the overall result of the reconciliation in the deferred
	// block at the very end.
	conditions.Delete(obj, meta.ReadyCondition)

	return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, nil
}

func (r *LocalizationReconciler) getDigestFromSource(ctx context.Context, sourceRef *meta.NamespacedObjectKindReference) (string, error) {
	switch sourceRef.Kind {
	case "Snapshot":
		snapshot := &v1alpha1.Snapshot{}
		if err := r.Get(ctx, types.NamespacedName{
			Namespace: sourceRef.Namespace,
			Name:      sourceRef.Name,
		}, snapshot); err != nil {
			return "", err
		}

		return snapshot.Status.LastReconciledDigest, nil
	case "OCIRepository", "GitRepository", "Bucket":
		return r.getArtifactChecksum(ctx, sourceRef)
	default:
		return "", fmt.Errorf("kind not supported for source object: %s", sourceRef.Kind)
	}
}

func (r *LocalizationReconciler) getArtifactChecksum(ctx context.Context, sourceRef *meta.NamespacedObjectKindReference) (string, error) {
	var source client.Object

	switch sourceRef.Kind {
	case v1beta2.OCIRepositoryKind:
		source = &v1beta2.OCIRepository{}
	case v1beta2.GitRepositoryKind:
		source = &v1beta2.GitRepository{}
	case v1beta2.BucketKind:
		source = &v1beta2.Bucket{}
	}
	if err := r.Get(ctx, types.NamespacedName{
		Namespace: sourceRef.Namespace,
		Name:      sourceRef.Name,
	}, source); err != nil {
		return "", err
	}

	obj, ok := source.(v1beta2.Source)
	if !ok {
		return "", fmt.Errorf("not a source: %v", obj)
	}

	return obj.GetArtifact().Checksum, nil
}

func (r *LocalizationReconciler) requestsForRevisionChangeOf(indexKey string) func(obj client.Object) []reconcile.Request {
	return func(obj client.Object) []reconcile.Request {
		snap, ok := obj.(*v1alpha1.Snapshot)
		if !ok {
			panic(fmt.Sprintf("expected snapshot but got: %T", obj))
		}

		if snap.Status.LastReconciledDigest == "" {
			return nil
		}

		// Get the Owner and if the Owner is the Localization object that I'm interested in,
		// that's my Snapshot.
		ctx := context.Background()
		var list v1alpha1.LocalizationList
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

func (r *LocalizationReconciler) indexBy(kind, field string) func(o client.Object) []string {
	return func(o client.Object) []string {
		l, ok := o.(*v1alpha1.Localization)
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
