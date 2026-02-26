package controllers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	"github.com/open-component-model/ocm-controller/pkg/cache"
	"github.com/open-component-model/ocm-controller/pkg/metrics"
	"github.com/open-component-model/ocm-controller/pkg/status"
	mh "github.com/open-component-model/pkg/metrics"
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

	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/patch"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/pkg/event"
	"github.com/open-component-model/ocm-controller/pkg/ocm"
)

// FluxDeployerReconciler reconciles a FluxDeployer object.
type FluxDeployerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	kuberecorder.EventRecorder
	ReconcileInterval   time.Duration
	RegistryServiceName string
	RetryInterval       time.Duration
	DynamicClient       dynamic.Interface

	CertSecretName string
	Cache          cache.Cache
}

// +kubebuilder:rbac:groups=delivery.ocm.software,resources=fluxdeployers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=delivery.ocm.software,resources=fluxdeployers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=delivery.ocm.software,resources=fluxdeployers/finalizers,verbs=update
// +kubebuilder:rbac:groups=delivery.ocm.software,resources=snapshots,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=source.toolkit.fluxcd.io,resources=ocirepositories;helmrepositories,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kustomize.toolkit.fluxcd.io,resources=kustomizations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=helm.toolkit.fluxcd.io,resources=helmreleases,verbs=get;list;watch;create;update;patch;delete

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
		ns := obj.Spec.SourceRef.Namespace
		if ns == "" {
			ns = obj.GetNamespace()
		}

		return []string{fmt.Sprintf("%s/%s", ns, obj.Spec.SourceRef.Name)}
	}); err != nil {
		return fmt.Errorf("failed setting index fields: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.FluxDeployer{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(
			&v1alpha1.Snapshot{},
			handler.EnqueueRequestsFromMapFunc(r.findObjects(sourceKey)),
			builder.WithPredicates(SnapshotDigestChangedPredicate{}),
		).
		Complete(r)
}

func (r *FluxDeployerReconciler) Reconcile(
	ctx context.Context,
	req ctrl.Request,
) (_ ctrl.Result, err error) {
	obj := &v1alpha1.FluxDeployer{}
	if err := r.Client.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to get deployer object: %w", err)
	}

	patchHelper := patch.NewSerialPatcher(obj, r.Client)

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

		if perr := patchHelper.Patch(ctx, obj); perr != nil {
			err = errors.Join(err, perr)
		}

		if err != nil {
			metrics.FluxDeployerReconcileFailed.WithLabelValues(obj.Name).Inc()
		}
	}()

	return r.reconcile(ctx, obj)
}

func (r *FluxDeployerReconciler) reconcile(
	ctx context.Context,
	obj *v1alpha1.FluxDeployer,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("reconciling flux-deployer", "name", obj.GetName())

	// get snapshot
	snapshot, err := r.getSnapshot(ctx, obj)
	if err != nil {
		logger.Info("could not find source ref", "name", obj.Spec.SourceRef.Name, "err", err)

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
	if obj.Spec.KustomizationTemplate != nil && obj.Spec.HelmReleaseTemplate != nil {
		return ctrl.Result{}, fmt.Errorf(
			"can't define both kustomization template and helm release template",
		)
	}

	// create kustomization
	if obj.Spec.KustomizationTemplate != nil {
		// can't check for helm content as we don't know where things are or what content to check for
		if err := r.createKustomizationSources(ctx, obj, snapshotURL, snapshot.Spec.Tag); err != nil {
			msg := "failed to create kustomization sources"
			logger.Error(err, msg)
			conditions.MarkFalse(
				obj,
				meta.ReadyCondition,
				v1alpha1.CreateOrUpdateKustomizationFailedReason,
				err.Error(),
				[]any{}...,
			)
			conditions.MarkStalled(
				obj,
				v1alpha1.CreateOrUpdateKustomizationFailedReason,
				err.Error(),
				[]any{}...,
			)
			event.New(r.EventRecorder, obj, nil, eventv1.EventSeverityError, msg)

			return ctrl.Result{}, err
		}
	}

	if obj.Spec.HelmReleaseTemplate != nil {
		tag := snapshot.Spec.Tag
		if v, ok := snapshot.Spec.Identity[v1alpha1.ResourceHelmChartVersion]; ok {
			tag = v
		}

		if err := r.createHelmSources(ctx, obj, snapshotURL, tag); err != nil {
			msg := "failed to create helm sources"
			logger.Error(err, msg)
			conditions.MarkFalse(
				obj,
				meta.ReadyCondition,
				v1alpha1.CreateOrUpdateHelmFailedReason,
				err.Error(),
				[]any{}...,
			)
			conditions.MarkStalled(obj, v1alpha1.CreateOrUpdateHelmFailedReason, err.Error(), []any{}...)
			event.New(r.EventRecorder, obj, nil, eventv1.EventSeverityError, msg)

			return ctrl.Result{}, err
		}
	}

	// if wait for ready, make sure all created objects are ready and existing.
	if obj.Spec.WaitForReady {
		var objs []conditions.Getter

		if err := r.findHelmRelease(ctx, obj, &objs); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to find helm release: %w", err)
		}

		if err := r.findOCIRepository(ctx, obj, &objs); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to find oci repository: %w", err)
		}

		if err := r.findKustomization(ctx, obj, &objs); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to find kustomization: %w", err)
		}

		for _, o := range objs {
			if !conditions.IsReady(o) {
				conditions.MarkFalse(
					obj,
					meta.ReadyCondition,
					v1alpha1.CreatedObjectsNotReadyReason,
					"waiting for ready condition on created sources",
				)

				return ctrl.Result{RequeueAfter: r.RetryInterval}, nil
			}
		}
	}

	status.MarkReady(r.EventRecorder, obj, "FluxDeployer '%s' is ready", obj.Name)

	metrics.FluxDeployerReconcileSuccess.WithLabelValues(obj.Name).Inc()

	if product := IsProductOwned(obj); product != "" {
		metrics.MPASDeployerReconciledStatus.WithLabelValues(product, mh.MPASStatusSuccess).Inc()
	}

	return ctrl.Result{}, nil
}

func (r *FluxDeployerReconciler) createKustomizationSources(
	ctx context.Context,
	obj *v1alpha1.FluxDeployer,
	url, tag string,
) error {
	// create oci registry
	if err := r.reconcileOCIRepo(ctx, obj, url, tag); err != nil {
		return fmt.Errorf("failed to create OCI repository: %w", err)
	}

	if err := r.reconcileKustomization(ctx, obj); err != nil {
		return fmt.Errorf("failed to create Kustomization object :%w", err)
	}

	return nil
}

func (r *FluxDeployerReconciler) createHelmSources(
	ctx context.Context,
	obj *v1alpha1.FluxDeployer,
	url, tag string,
) error {
	// create oci registry
	if err := r.reconcileOCIRepo(ctx, obj, url, tag); err != nil {
		return fmt.Errorf("failed to create OCI repository: %w", err)
	}

	if err := r.reconcileHelmRelease(ctx, obj); err != nil {
		return fmt.Errorf("failed to create Helm Release object :%w", err)
	}

	return nil
}

func (r *FluxDeployerReconciler) reconcileOCIRepo(
	ctx context.Context,
	obj *v1alpha1.FluxDeployer,
	url, tag string,
) error {
	ociRepoCR := &sourcev1.OCIRepository{
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
		ociRepoCR.Spec = sourcev1.OCIRepositorySpec{
			Interval: obj.Spec.Interval,
			CertSecretRef: &meta.LocalObjectReference{
				Name: r.CertSecretName,
			},
			URL: url,
			Reference: &sourcev1.OCIRepositoryRef{
				Tag: tag,
			},
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create reconcile oci repo: %w", err)
	}

	obj.Status.OCIRepository = ociRepoCR.GetNamespace() + "/" + ociRepoCR.GetName()

	return nil
}

func (r *FluxDeployerReconciler) reconcileKustomization(
	ctx context.Context,
	obj *v1alpha1.FluxDeployer,
) error {
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
		kust.Spec = *obj.Spec.KustomizationTemplate
		kust.Spec.SourceRef.Kind = sourcev1.OCIRepositoryKind
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

func (r *FluxDeployerReconciler) getSnapshot(
	ctx context.Context,
	obj *v1alpha1.FluxDeployer,
) (*v1alpha1.Snapshot, error) {
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

func (r *FluxDeployerReconciler) findObjects(sourceKey string) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		var selectorTerm string
		switch obj.(type) {
		case *v1alpha1.Snapshot:
			if len(obj.GetOwnerReferences()) != 1 {
				return []reconcile.Request{}
			}
			selectorTerm = fmt.Sprintf("%s/%s", obj.GetNamespace(), obj.GetOwnerReferences()[0].Name)
		default:
			return []reconcile.Request{}
		}

		sourceRefs := &v1alpha1.FluxDeployerList{}
		if err := r.List(ctx, sourceRefs, &client.ListOptions{
			FieldSelector: fields.OneTermEqualSelector(sourceKey, selectorTerm),
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

func (r *FluxDeployerReconciler) reconcileHelmRelease(
	ctx context.Context,
	obj *v1alpha1.FluxDeployer,
) error {
	helmRelease := &helmv2.HelmRelease{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: obj.GetNamespace(),
			Name:      obj.GetName(),
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, helmRelease, func() error {
		if helmRelease.ObjectMeta.CreationTimestamp.IsZero() {
			if err := controllerutil.SetOwnerReference(obj, helmRelease, r.Scheme); err != nil {
				return fmt.Errorf("failed to set owner reference on oci repository source: %w", err)
			}
		}
		helmRelease.Spec = *obj.Spec.HelmReleaseTemplate
		helmRelease.Spec.ChartRef = &helmv2.CrossNamespaceSourceReference{
			Kind:      "OCIRepository",
			Name:      obj.GetName(),
			Namespace: obj.GetNamespace(),
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create reconcile kustomization: %w", err)
	}

	obj.Status.HelmRelease = helmRelease.GetNamespace() + "/" + helmRelease.GetName()

	return nil
}

func (r *FluxDeployerReconciler) findHelmRelease(ctx context.Context, obj *v1alpha1.FluxDeployer, objs *[]conditions.Getter) error {
	if obj.Status.HelmRelease == "" {
		return nil
	}

	helmRelease := &helmv2.HelmRelease{}
	split := strings.Split(obj.Status.HelmRelease, "/")
	if len(split) == 0 || len(split) != 2 {
		return fmt.Errorf("failed to find helm release in status: %s", obj.Status.HelmRelease)
	}

	if err := r.Client.Get(ctx, client.ObjectKey{Namespace: split[0], Name: split[1]}, helmRelease); err != nil {
		return fmt.Errorf("failed to find helm release: %w", err)
	}

	*objs = append(*objs, helmRelease)

	return nil
}

func (r *FluxDeployerReconciler) findOCIRepository(ctx context.Context, obj *v1alpha1.FluxDeployer, objs *[]conditions.Getter) error {
	if obj.Status.OCIRepository == "" {
		return nil
	}

	ociRepo := &sourcev1.OCIRepository{}
	split := strings.Split(obj.Status.OCIRepository, "/")
	if len(split) == 0 || len(split) != 2 {
		return fmt.Errorf("failed to find oci repository in status: %s", obj.Status.OCIRepository)
	}

	if err := r.Client.Get(ctx, client.ObjectKey{Namespace: split[0], Name: split[1]}, ociRepo); err != nil {
		return fmt.Errorf("failed to find oci repository: %w", err)
	}

	*objs = append(*objs, ociRepo)

	return nil
}

func (r *FluxDeployerReconciler) findKustomization(ctx context.Context, obj *v1alpha1.FluxDeployer, objs *[]conditions.Getter) error {
	if obj.Status.Kustomization == "" {
		return nil
	}

	kustomization := &kustomizev1.Kustomization{}
	split := strings.Split(obj.Status.Kustomization, "/")
	if len(split) == 0 || len(split) != 2 {
		return fmt.Errorf("failed to find kustomization in status: %s", obj.Status.Kustomization)
	}

	if err := r.Client.Get(ctx, client.ObjectKey{Namespace: split[0], Name: split[1]}, kustomization); err != nil {
		return fmt.Errorf("failed to find kustomization: %w", err)
	}

	*objs = append(*objs, kustomization)

	return nil
}
