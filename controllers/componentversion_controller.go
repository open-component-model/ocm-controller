package controllers

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/Masterminds/semver/v3"
	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/patch"
	rreconcile "github.com/fluxcd/pkg/runtime/reconcile"
	mh "github.com/open-component-model/pkg/metrics"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kuberecorder "k8s.io/client-go/tools/record"
	"ocm.software/ocm/api/ocm"
	ocmdesc "ocm.software/ocm/api/ocm/compdesc"
	compdesc "ocm.software/ocm/api/ocm/compdesc/versions/ocm.software/v3alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/open-component-model/ocm-controller/pkg/metrics"
	"github.com/open-component-model/ocm-controller/pkg/status"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/pkg/component"
	"github.com/open-component-model/ocm-controller/pkg/event"
	ocmclient "github.com/open-component-model/ocm-controller/pkg/ocm"
)

// ComponentVersionReconciler reconciles a ComponentVersion object.
type ComponentVersionReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	kuberecorder.EventRecorder

	OCMClient ocmclient.Contract
}

//+kubebuilder:rbac:groups=delivery.ocm.software,resources=componentversions;componentdescriptors,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=delivery.ocm.software,resources=componentversions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=delivery.ocm.software,resources=componentversions/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=services;pods,verbs=get;create;update;patch;delete
//+kubebuilder:rbac:groups="apps",resources=deployments,verbs=get;create;update;patch;delete

// +kubebuilder:rbac:groups="",resources=secrets;configmaps;serviceaccounts,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=serviceaccounts/token,verbs=create
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// SetupWithManager sets up the controller with the Manager.
func (r *ComponentVersionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	const (
		sourceKey = ".metadata.repository.secretRef"
	)

	if err := mgr.GetFieldIndexer().IndexField(context.TODO(), &v1alpha1.ComponentVersion{}, sourceKey, func(rawObj client.Object) []string {
		obj, ok := rawObj.(*v1alpha1.ComponentVersion)
		if !ok {
			return []string{}
		}
		if obj.Spec.Repository.SecretRef == nil {
			return []string{}
		}

		ns := obj.GetNamespace()

		return []string{fmt.Sprintf("%s/%s", ns, obj.Spec.Repository.SecretRef.Name)}
	}); err != nil {
		return fmt.Errorf("failed setting index fields: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ComponentVersion{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.findObjects(sourceKey))).
		Complete(r)
}

// findObjects finds component versions that have a key for the secret that triggered this watch event.
func (r *ComponentVersionReconciler) findObjects(key string) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		list := &v1alpha1.ComponentVersionList{}
		if err := r.List(ctx, list, &client.ListOptions{
			FieldSelector: fields.OneTermEqualSelector(key, client.ObjectKeyFromObject(obj).String()),
		}); err != nil {
			return []reconcile.Request{}
		}

		requests := make([]reconcile.Request, len(list.Items))
		for i, item := range list.Items {
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

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ComponentVersionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, retErr error) {
	logger := log.FromContext(ctx).WithName("ocm-component-version-reconcile")

	logger.Info("starting ocm component loop")

	obj := &v1alpha1.ComponentVersion{}
	if err := r.Client.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to get component object: %w", err)
	}

	if obj.Spec.Suspend {
		logger.Info("component object suspended")

		return ctrl.Result{}, nil
	}

	patchHelper := patch.NewSerialPatcher(obj, r.Client)

	// Always attempt to patch the object and status after each reconciliation.
	defer func() {
		if derr := status.UpdateStatus(ctx, patchHelper, obj, r.EventRecorder, obj.GetRequeueAfter(), retErr); derr != nil {
			retErr = errors.Join(retErr, derr)
		}

		if retErr != nil {
			metrics.ComponentVersionReconcileFailed.WithLabelValues(obj.Spec.Component).Inc()
		}
	}()

	// Starts the progression by setting ReconcilingCondition. This will be checked in defer.
	// Should only be deleted on a success.
	rreconcile.ProgressiveStatus(false, obj, meta.ProgressingReason, "reconciliation in progress for component: %s", obj.Spec.Component)

	octx, err := r.OCMClient.CreateAuthenticatedOCMContext(ctx, obj)
	if err != nil {
		// we don't fail here, because all manifests might have been applied at once or the secret
		// for authentication is being reconciled.
		status.MarkAsStalled(
			r.EventRecorder,
			obj,
			v1alpha1.AuthenticatedContextCreationFailedReason,
			fmt.Sprintf("authentication failed for repository: %s with error: %s", obj.Spec.Repository.URL, err),
		)
		metrics.ComponentVersionReconcileFailed.WithLabelValues(obj.Spec.Component).Inc()

		return ctrl.Result{}, nil
	}

	// reconcile the version before calling reconcile func
	update, version, err := r.checkVersion(ctx, octx, obj)
	if err != nil {
		// The component might not be there yet. We don't fail but keep polling instead.
		status.MarkNotReady(
			r.EventRecorder,
			obj,
			v1alpha1.CheckVersionFailedReason,
			fmt.Sprintf("version check failed for %s %s with error: %s", obj.Spec.Component, obj.Spec.Version.Semver, err),
		)
		metrics.ComponentVersionReconcileFailed.WithLabelValues(obj.Spec.Component).Inc()

		return ctrl.Result{
			RequeueAfter: obj.GetRequeueAfter(),
		}, nil
	}

	if !update {
		status.MarkReady(r.EventRecorder, obj, "Applied version: %s", version)

		return ctrl.Result{
			RequeueAfter: obj.GetRequeueAfter(),
		}, nil
	}

	rreconcile.ProgressiveStatus(false, obj, meta.ProgressingReason, "updating component to new version: %s: %s", obj.Spec.Component, version)

	ok, err := r.OCMClient.VerifyComponent(ctx, octx, obj, version)
	if err != nil {
		status.MarkNotReady(
			r.EventRecorder,
			obj,
			v1alpha1.VerificationFailedReason,
			fmt.Sprintf("failed to verify %s with constraint %s with error: %s", obj.Spec.Component, obj.Spec.Version.Semver, err),
		)
		metrics.ComponentVersionReconcileFailed.WithLabelValues(obj.Spec.Component).Inc()

		return ctrl.Result{
			RequeueAfter: obj.GetRequeueAfter(),
		}, nil
	}

	if !ok {
		status.MarkNotReady(
			r.EventRecorder,
			obj,
			v1alpha1.VerificationFailedReason,
			"attempted to verify component, but the digest didn't match",
		)
		metrics.ComponentVersionReconcileFailed.WithLabelValues(obj.Spec.Component).Inc()

		return ctrl.Result{
			RequeueAfter: obj.GetRequeueAfter(),
		}, nil
	}

	return r.reconcile(ctx, octx, obj, version)
}

func (r *ComponentVersionReconciler) reconcile(
	ctx context.Context,
	octx ocm.Context,
	obj *v1alpha1.ComponentVersion,
	version string,
) (ctrl.Result, error) {
	if obj.Generation != obj.Status.ObservedGeneration {
		rreconcile.ProgressiveStatus(false, obj, meta.ProgressingReason,
			"processing object: new generation %d -> %d", obj.Status.ObservedGeneration, obj.Generation)
	}

	obj.Status.ReplicatedRepositoryURL = obj.Spec.Repository.URL

	// Get the component version from the original repository URL.
	cv, err := r.OCMClient.GetComponentVersion(ctx, octx, obj.GetRepositoryURL(), obj.Spec.Component, version)
	if err != nil {
		err = fmt.Errorf("failed to get component version: %w", err)
		status.MarkNotReady(
			r.EventRecorder,
			obj,
			v1alpha1.ComponentVersionInvalidReason,
			err.Error(),
		)

		return ctrl.Result{}, err
	}

	defer cv.Close()

	// If there is a transfer requested, transfer the cv to that location.
	if obj.Spec.Destination != nil {
		rreconcile.ProgressiveStatus(false, obj, meta.ProgressingReason, "transferring component to target repository: %s", obj.Spec.Destination.URL)

		if err := r.OCMClient.TransferComponent(octx, obj, cv); err != nil {
			err := fmt.Errorf("failed to transfer components: %w", err)
			status.MarkNotReady(r.EventRecorder, obj, v1alpha1.TransferFailedReason, err.Error())

			return ctrl.Result{}, err
		}

		// set the new URL to the destination URL
		obj.Status.ReplicatedRepositoryURL = obj.Spec.Destination.URL

		// update the ocm component version to be the new version from the replicated destination
		cv, err = r.OCMClient.GetComponentVersion(ctx, octx, obj.GetRepositoryURL(), obj.Spec.Component, version)
		if err != nil {
			err = fmt.Errorf("failed to get transferred component version: %w", err)
			status.MarkNotReady(
				r.EventRecorder,
				obj,
				v1alpha1.ComponentVersionInvalidReason,
				err.Error(),
			)

			return ctrl.Result{}, err
		}

		defer cv.Close()
	}

	cd, descriptor, err := r.createInitialComponentDescriptor(obj, cv)
	if err != nil {
		err = fmt.Errorf("failed to create initial component descriptor: %w", err)
		status.MarkNotReady(
			r.EventRecorder,
			obj,
			v1alpha1.ConvertComponentDescriptorFailedReason,
			err.Error(),
		)
	}

	// create or update the component descriptor kubernetes resource
	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, descriptor, func() error {
		if descriptor.ObjectMeta.CreationTimestamp.IsZero() {
			if err := controllerutil.SetOwnerReference(obj, descriptor, r.Scheme); err != nil {
				return fmt.Errorf("failed to set owner reference: %w", err)
			}
		}

		componentDescriptor, ok := cd.(*compdesc.ComponentDescriptor)
		if !ok {
			return fmt.Errorf("object was not a component descriptor but was: %v", cd)
		}

		spec := v1alpha1.ComponentDescriptorSpec{
			ComponentVersionSpec: componentDescriptor.Spec,
			Version:              cd.GetVersion(),
		}
		descriptor.Spec = spec

		return nil
	})
	if err != nil {
		err = fmt.Errorf("failed to create or update component descriptor: %w", err)
		status.MarkNotReady(
			r.EventRecorder,
			obj,
			v1alpha1.CreateOrUpdateComponentDescriptorFailedReason,
			err.Error(),
		)

		return ctrl.Result{}, err
	}

	componentDescriptor := v1alpha1.Reference{
		Name:    cd.GetName(),
		Version: cd.GetVersion(),
		ComponentDescriptorRef: meta.NamespacedObjectReference{
			Name:      descriptor.GetName(),
			Namespace: descriptor.GetNamespace(),
		},
	}

	rreconcile.ProgressiveStatus(false, obj, meta.ProgressingReason, "descriptors created, expanding references")

	desc := cv.GetDescriptor()
	if desc == nil {
		return ctrl.Result{}, fmt.Errorf("no descriptor found for component version %s:%s", cv.GetName(), cv.GetVersion())
	}

	// build up component reference graph
	componentDescriptor.References, err = r.parseReferences(ctx, octx, obj, desc.References)
	if err != nil {
		err = fmt.Errorf("failed to parse references: %w", err)
		status.MarkNotReady(
			r.EventRecorder,
			obj,
			v1alpha1.ParseReferencesFailedReason,
			err.Error(),
		)

		return ctrl.Result{}, err
	}

	obj.Status.ComponentDescriptor = componentDescriptor
	obj.Status.ReconciledVersion = version

	metrics.ComponentVersionReconciledTotal.WithLabelValues(cv.GetName(), cv.GetVersion()).Inc()

	if product := IsProductOwned(obj); product != "" {
		metrics.MPASComponentVersionReconciledStatus.WithLabelValues(product, mh.MPASStatusSuccess).Inc()
	}

	status.MarkReady(r.EventRecorder, obj, "Applied version: %s", version)

	return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, nil
}

func (r *ComponentVersionReconciler) checkVersion(ctx context.Context, octx ocm.Context, obj *v1alpha1.ComponentVersion) (bool, string, error) {
	logger := log.FromContext(ctx).WithName("ocm-component-version-reconcile")

	latest, err := r.OCMClient.GetLatestValidComponentVersion(ctx, octx, obj)
	if err != nil {
		return false, "", fmt.Errorf("failed to get latest component version: %w", err)
	}
	logger.V(v1alpha1.LevelDebug).Info("got latest version of component", "version", latest)

	latestSemver, err := semver.NewVersion(latest)
	if err != nil {
		return false, "", fmt.Errorf("failed to parse latest version: %w", err)
	}

	reconciledVersion := "0.0.0"
	if obj.Status.ReconciledVersion != "" {
		reconciledVersion = obj.Status.ReconciledVersion
	}
	current, err := semver.NewVersion(reconciledVersion)
	if err != nil {
		return false, "", fmt.Errorf("failed to parse reconciled version: %w", err)
	}
	logger.V(v1alpha1.LevelDebug).Info("current reconciled version is", "reconciled", current.String())

	event.New(
		r.EventRecorder,
		obj,
		nil,
		eventv1.EventSeverityInfo,
		"Version check succeeded, found latest version: %s",
		latest,
	)

	constraint, err := semver.NewConstraint(obj.Spec.Version.Semver)
	if err != nil {
		return false, "", fmt.Errorf("failed to parse constraint version: %w", err)
	}

	if current.LessThan(latestSemver) || !constraint.Check(current) {
		return true, latest, nil
	}

	return false, "", nil
}

// parseReferences takes a list of references to embedded components and constructs a dependency tree out of them.
// It recursively calls itself, constructing a tree of referenced components.
// For each referenced component a ComponentDescriptor custom resource will be created.
func (r *ComponentVersionReconciler) parseReferences(
	ctx context.Context,
	octx ocm.Context,
	parent *v1alpha1.ComponentVersion,
	references ocmdesc.References,
) ([]v1alpha1.Reference, error) {
	result := make([]v1alpha1.Reference, 0)
	for _, ref := range references {
		reference, err := r.constructComponentDescriptorsForReference(ctx, octx, parent, ref)
		if err != nil {
			return nil, fmt.Errorf("failed to construct component descriptor: %w", err)
		}
		result = append(result, *reference)
	}

	return result, nil
}

func (r *ComponentVersionReconciler) constructComponentDescriptorsForReference(
	ctx context.Context,
	octx ocm.Context,
	parent *v1alpha1.ComponentVersion,
	ref ocmdesc.Reference,
) (*v1alpha1.Reference, error) {
	// get component version
	rcv, err := r.OCMClient.GetComponentVersion(ctx, octx, parent.GetRepositoryURL(), ref.ComponentName, ref.Version)
	if err != nil {
		return nil, fmt.Errorf("failed to get component version: %w", err)
	}
	defer rcv.Close()

	descriptor, err := r.createComponentDescriptor(ctx, rcv, parent, ref)
	if err != nil {
		return nil, fmt.Errorf("failed to create component descriptor: %w", err)
	}

	reference := v1alpha1.Reference{
		Name:    ref.Name,
		Version: ref.Version,
		ComponentDescriptorRef: meta.NamespacedObjectReference{
			Name:      descriptor.Name,
			Namespace: descriptor.Namespace,
		},
		ExtraIdentity: ref.ExtraIdentity,
	}

	desc := rcv.GetDescriptor()
	if desc == nil {
		return nil, fmt.Errorf("no descriptor found for component version %s:%s", rcv.GetName(), rcv.GetVersion())
	}

	if len(desc.References) > 0 {
		// recursively call parseReference on the embedded references in the new descriptor.
		out, err := r.parseReferences(ctx, octx, parent, desc.References)
		if err != nil {
			return nil, err
		}

		reference.References = out
	}

	return &reference, nil
}

func (r *ComponentVersionReconciler) createComponentDescriptor(
	ctx context.Context,
	rcv ocm.ComponentVersionAccess,
	parent *v1alpha1.ComponentVersion,
	ref ocmdesc.Reference,
) (*v1alpha1.ComponentDescriptor, error) {
	// convert ComponentDescriptor to v3alpha1
	dv := &compdesc.DescriptorVersion{}
	cd, err := dv.ConvertFrom(rcv.GetDescriptor())
	if err != nil {
		return nil, fmt.Errorf("failed to convert component descriptor: %w", err)
	}

	// setup the component descriptor kubernetes resource
	componentName, err := component.ConstructUniqueName(ref.ComponentName, ref.Version, ref.GetMeta().GetExtraIdentity())
	if err != nil {
		return nil, fmt.Errorf("failed to generate name: %w", err)
	}

	componentDescriptor, ok := cd.(*compdesc.ComponentDescriptor)
	if !ok {
		return nil, fmt.Errorf("object was not a component descriptor: %v", cd)
	}

	labels := componentDescriptor.GetLabels()
	labelMap := make(map[string]string)
	for k, v := range labels.AsMap() {
		switch t := v.(type) {
		case string:
			labelMap[k] = t
		case float64:
			labelMap[k] = strconv.FormatFloat(t, 'f', -1, 64)
		case bool:
			labelMap[k] = strconv.FormatBool(t)
		case int:
			labelMap[k] = strconv.Itoa(t)
		}
	}

	descriptor := &v1alpha1.ComponentDescriptor{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: parent.GetNamespace(),
			Name:      componentName,
			Labels:    labelMap,
		},
		Spec: v1alpha1.ComponentDescriptorSpec{
			ComponentVersionSpec: componentDescriptor.Spec,
			Version:              ref.Version,
		},
	}

	// create or update the component descriptor kubernetes resource
	// we don't need to update it
	if _, err = controllerutil.CreateOrUpdate(ctx, r.Client, descriptor, func() error {
		if descriptor.ObjectMeta.CreationTimestamp.IsZero() {
			if err := controllerutil.SetOwnerReference(parent, descriptor, r.Scheme); err != nil {
				return fmt.Errorf("failed to set owner reference: %w", err)
			}
		}

		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to create/update component descriptor: %w", err)
	}

	return descriptor, nil
}

func (r *ComponentVersionReconciler) createInitialComponentDescriptor(
	obj *v1alpha1.ComponentVersion,
	cv ocm.ComponentVersionAccess,
) (ocmdesc.ComponentDescriptorVersion, *v1alpha1.ComponentDescriptor, error) {
	// convert ComponentDescriptor to v3alpha1
	dv := &compdesc.DescriptorVersion{}
	cd, err := dv.ConvertFrom(cv.GetDescriptor())
	if err != nil {
		return nil, nil, err
	}

	rreconcile.ProgressiveStatus(false, obj, meta.ProgressingReason, "component fetched, creating descriptors")

	// setup the component descriptor kubernetes resource
	componentName, err := component.ConstructUniqueName(cd.GetName(), cd.GetVersion(), nil)
	if err != nil {
		return nil, nil, err
	}

	labels := cv.GetDescriptor().GetLabels()
	labelMap := make(map[string]string)
	for k, v := range labels.AsMap() {
		switch t := v.(type) {
		case string:
			labelMap[k] = t
		case float64:
			labelMap[k] = strconv.FormatFloat(t, 'f', -1, 64)
		case bool:
			labelMap[k] = strconv.FormatBool(t)
		case int:
			labelMap[k] = strconv.Itoa(t)
		}
	}

	descriptor := &v1alpha1.ComponentDescriptor{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: obj.GetNamespace(),
			Name:      componentName,
			Labels:    labelMap,
		},
	}

	return cd, descriptor, nil
}
