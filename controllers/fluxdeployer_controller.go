// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"errors"
	"fmt"
	"time"

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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1beta2"
	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/patch"
	sourcev1beta2 "github.com/fluxcd/source-controller/api/v1beta2"
	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	deliveryv1alpha1 "github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/pkg/event"
	"github.com/open-component-model/ocm-controller/pkg/ocm"
)

// FluxDeployerReconciler reconciles a FluxDeployer object
type FluxDeployerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	kuberecorder.EventRecorder
	ReconcileInterval   time.Duration
	RegistryServiceName string
	RetryInterval       time.Duration
	DynamicClient       dynamic.Interface
}

// +kubebuilder:rbac:groups=delivery.ocm.software,resources=fluxdeployers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=delivery.ocm.software,resources=fluxdeployers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=delivery.ocm.software,resources=fluxdeployers/finalizers,verbs=update

// +kubebuilder:rbac:groups=delivery.ocm.software,resources=snapshots,verbs=get;list;watch;create;update;patch;delete

// +kubebuilder:rbac:groups=source.toolkit.fluxcd.io,resources=ocirepositories,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kustomize.toolkit.fluxcd.io,resources=kustomizations,verbs=get;list;watch;create;update;patch;delete

// SetupWithManager sets up the controller with the Manager.
func (r *FluxDeployerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	const (
		sourceKey = ".metadata.source"
	)

	if err := mgr.GetFieldIndexer().IndexField(context.TODO(), &v1alpha1.FluxDeployer{}, sourceKey, func(rawObj client.Object) []string {
		obj, ok := rawObj.(*v1alpha1.FluxDeployer)
		if !ok {
			return []string{}
		}
		return []string{obj.Spec.SourceRef.Name}
	}); err != nil {
		return fmt.Errorf("failed setting index fields: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&deliveryv1alpha1.FluxDeployer{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(
			&source.Kind{Type: &v1alpha1.Snapshot{}},
			handler.EnqueueRequestsFromMapFunc(r.findObjects(sourceKey)),
			builder.WithPredicates(SnapshotDigestChangedPredicate{}),
		).
		Complete(r)
}

func (r *FluxDeployerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	obj := &v1alpha1.FluxDeployer{}
	if err := r.Client.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get deployer object: %w", err)
	}

	patchHelper, err := patch.NewHelper(obj, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Always attempt to patch the object and status after each reconciliation.
	defer func() {
		// Patching has not been set up, or the controller errored earlier.
		if patchHelper == nil {
			return
		}

		// Set status observed generation option if the object is stalled or ready.
		if conditions.IsStalled(obj) || conditions.IsReady(obj) {
			obj.Status.ObservedGeneration = obj.Generation
		}

		if err := patchHelper.Patch(ctx, obj); err != nil {
			return
		}
	}()

	return r.reconcile(ctx, obj)
}

func (r *FluxDeployerReconciler) reconcile(ctx context.Context, obj *v1alpha1.FluxDeployer) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("reconciling flux-deployer", "name", obj.GetName())

	// get snapshot
	snapshot, err := r.getSnapshot(ctx, obj)
	if err != nil {
		logger.Info("could not find source ref", "name", obj.Spec.SourceRef.Name)
		return ctrl.Result{RequeueAfter: r.RetryInterval}, nil
	}

	// requeue if snapshot is not ready
	if conditions.IsFalse(snapshot, meta.ReadyCondition) {
		logger.Info("snapshot not ready yet", "snapshot", snapshot.Name)
		return ctrl.Result{RequeueAfter: r.RetryInterval}, nil
	}

	snapshotRepo, err := ocm.ConstructRepositoryName(snapshot.Spec.Identity)
	if err != nil {
		return ctrl.Result{}, err
	}

	snapshotURL := fmt.Sprintf("oci://%s/%s", r.RegistryServiceName, snapshotRepo)

	// create oci registry
	if err := r.reconcileOCIRepo(ctx, obj, snapshotURL, snapshot.Spec.Tag); err != nil {
		msg := "failed to create or update oci repository"
		logger.Error(err, msg)
		conditions.MarkFalse(obj, meta.ReadyCondition, v1alpha1.CreateOrUpdateOCIRepositoryFailedReason, err.Error())
		conditions.MarkStalled(obj, v1alpha1.CreateOrUpdateOCIRepositoryFailedReason, err.Error())
		event.New(r.EventRecorder, obj, eventv1.EventSeverityError, msg, nil)
		return ctrl.Result{}, err
	}

	// create kustomization
	if err = r.reconcileKustomization(ctx, obj); err != nil {
		msg := "failed to create or update kustomization"
		logger.Error(err, msg)
		conditions.MarkFalse(obj, meta.ReadyCondition, v1alpha1.CreateOrUpdateOCIRepositoryFailedReason, err.Error())
		conditions.MarkStalled(obj, v1alpha1.CreateOrUpdateOCIRepositoryFailedReason, err.Error())
		event.New(r.EventRecorder, obj, eventv1.EventSeverityError, msg, nil)
		return ctrl.Result{}, err
	}

	msg := fmt.Sprintf("FluxDeployer '%s' is ready", obj.Name)
	conditions.MarkTrue(obj, meta.ReadyCondition, meta.SucceededReason, msg)

	return ctrl.Result{}, nil
}

func (r *FluxDeployerReconciler) reconcileOCIRepo(ctx context.Context, obj *v1alpha1.FluxDeployer, url, tag string) error {
	ociRepoCR := &sourcev1beta2.OCIRepository{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: obj.GetNamespace(),
			Name:      obj.GetName(),
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, ociRepoCR, func() error {
		if ociRepoCR.ObjectMeta.CreationTimestamp.IsZero() {
			if err := controllerutil.SetOwnerReference(obj, ociRepoCR, r.Scheme); err != nil {
				return fmt.Errorf("failed to set owner reference on oci repository source: %w", err)
			}
		}
		ociRepoCR.Spec = sourcev1beta2.OCIRepositorySpec{
			Interval: obj.Spec.KustomizationTemplate.Interval,
			Insecure: true,
			URL:      url,
			Reference: &sourcev1beta2.OCIRepositoryRef{
				Tag: tag,
			},
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to create reconcile oci repo: %w", err)
	}

	return nil
}

func (r *FluxDeployerReconciler) reconcileKustomization(ctx context.Context, obj *v1alpha1.FluxDeployer) error {
	kust := &kustomizev1.Kustomization{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: obj.GetNamespace(),
			Name:      obj.GetName(),
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, kust, func() error {
		if kust.ObjectMeta.CreationTimestamp.IsZero() {
			if err := controllerutil.SetOwnerReference(obj, kust, r.Scheme); err != nil {
				return fmt.Errorf("failed to set owner reference on oci repository source: %w", err)
			}
		}
		kust.Spec = obj.Spec.KustomizationTemplate
		kust.Spec.SourceRef.Kind = sourcev1beta2.OCIRepositoryKind
		kust.Spec.SourceRef.Namespace = obj.GetNamespace()
		kust.Spec.SourceRef.Name = obj.GetName()
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to create reconcile kustomization: %w", err)
	}

	obj.Status.Kustomization = kust.GetNamespace() + "/" + kust.GetName()

	return nil
}

func (r *FluxDeployerReconciler) getSnapshot(ctx context.Context, obj *v1alpha1.FluxDeployer) (*v1alpha1.Snapshot, error) {
	if obj.Spec.SourceRef.APIVersion == "" {
		obj.Spec.SourceRef.APIVersion = v1alpha1.GroupVersion.String()
	}

	if obj.Spec.SourceRef.Namespace == "" {
		obj.Spec.SourceRef.Namespace = obj.GetNamespace()
	}

	ref := obj.Spec.SourceRef
	src, err := r.DynamicClient.
		Resource(ref.GetGVR()).
		Namespace(ref.Namespace).
		Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot: %w", err)
	}

	snapshotName, ok, err := unstructured.NestedString(src.Object, "status", "snapshotName")
	if err != nil {
		return nil, fmt.Errorf("failed get the get snapshot: %w", err)
	}
	if !ok {
		return nil, errors.New("snapshot name not found in status")
	}

	key := types.NamespacedName{
		Name:      snapshotName,
		Namespace: ref.Namespace,
	}

	snapshot := &v1alpha1.Snapshot{}
	if err := r.Get(ctx, key, snapshot); err != nil {
		return nil,
			fmt.Errorf("failed to get snapshot: %w", err)
	}

	return snapshot, nil
}

func (r *FluxDeployerReconciler) findObjects(sourceKey string) func(client.Object) []reconcile.Request {
	return func(obj client.Object) []reconcile.Request {
		var selectorTerm string
		switch obj.(type) {
		case *v1alpha1.Snapshot:
			if len(obj.GetOwnerReferences()) != 1 {
				return []reconcile.Request{}
			}
			selectorTerm = obj.GetOwnerReferences()[0].Name
		default:
			return []reconcile.Request{}
		}

		sourceRefs := &v1alpha1.FluxDeployerList{}
		if err := r.List(context.TODO(), sourceRefs, &client.ListOptions{
			FieldSelector: fields.OneTermEqualSelector(sourceKey, selectorTerm),
			Namespace:     obj.GetNamespace(),
		}); err != nil {
			return []reconcile.Request{}
		}

		requests := make([]reconcile.Request, len(sourceRefs.Items))
		for i, item := range sourceRefs.Items {
			requests[i] = reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: item.GetNamespace(),
					Name:      item.GetName(),
				},
			}
		}

		return requests
	}
}
