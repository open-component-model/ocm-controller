// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"errors"
	"fmt"

	"github.com/Masterminds/semver/v3"
	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/patch"
	rreconcile "github.com/fluxcd/pkg/runtime/reconcile"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kuberecorder "k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	ocmdesc "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc"
	compdesc "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/ocm.software/v3alpha1"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/pkg/component"
	"github.com/open-component-model/ocm-controller/pkg/event"
	ocmclient "github.com/open-component-model/ocm-controller/pkg/ocm"
)

// ComponentVersionReconciler reconciles a ComponentVersion object
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

// +kubebuilder:rbac:groups="",resources=secrets;serviceaccounts,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// SetupWithManager sets up the controller with the Manager.
func (r *ComponentVersionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ComponentVersion{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(r)
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
		return
	}

	patchHelper, err := patch.NewHelper(obj, r.Client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create patchhelper: %w", err)
	}

	// Always attempt to patch the object and status after each reconciliation.
	defer func() {
		// Patching has not been set up, or the controller errored earlier.
		if patchHelper == nil {
			return
		}

		// If still reconciling then reconciliation did not succeed, set to ProgressingWithRetry to
		// indicate that reconciliation will be retried.
		if conditions.IsReconciling(obj) {
			reconciling := conditions.Get(obj, meta.ReconcilingCondition)
			reconciling.Reason = meta.ProgressingWithRetryReason
			conditions.Set(obj, reconciling)
			msg := fmt.Sprintf("Reconciliation did not succeed, retrying in %s", obj.GetRequeueAfter())
			event.New(r.EventRecorder, obj, eventv1.EventSeverityError, msg, nil)
		}

		// Set status observed generation option if the component is ready.
		if conditions.IsReady(obj) {
			obj.Status.ObservedGeneration = obj.Generation
			msg := fmt.Sprintf("Reconciliation finished, next run in %s", obj.GetRequeueAfter())
			vid := fmt.Sprintf("%s:%s", obj.Status.ComponentDescriptor.Name, obj.Status.ReconciledVersion)
			metadata := make(map[string]string)
			metadata[v1alpha1.GroupVersion.Group+"/component_version"] = vid
			event.New(r.EventRecorder, obj, eventv1.EventSeverityInfo, msg, metadata)
		}

		// Update the object.
		if err := patchHelper.Patch(ctx, obj); err != nil {
			retErr = errors.Join(retErr, err)
		}
	}()

	rreconcile.ProgressiveStatus(false, obj, meta.ProgressingReason, "reconcilation in progress for component: %s", obj.Spec.Component)

	octx, err := r.OCMClient.CreateAuthenticatedOCMContext(ctx, obj)
	if err != nil {
		msg := fmt.Sprintf("authentication failed for repository: %s with error: %s", obj.Spec.Repository.URL, err)
		conditions.MarkFalse(obj, meta.ReadyCondition, v1alpha1.AuthenticatedContextCreationFailedReason, msg)
		event.New(r.EventRecorder, obj, eventv1.EventSeverityError, msg, nil)

		return ctrl.Result{
			RequeueAfter: obj.GetRequeueAfter(),
		}, nil
	}

	// reconcile the version before calling reconcile func
	update, version, err := r.checkVersion(ctx, octx, obj)
	if err != nil {
		msg := fmt.Sprintf("version check failed for %s %s with error: %s", obj.Spec.Component, obj.Spec.Version.Semver, err)
		conditions.MarkFalse(obj, meta.ReadyCondition, v1alpha1.CheckVersionFailedReason, msg)
		event.New(r.EventRecorder, obj, eventv1.EventSeverityError, msg, nil)

		return ctrl.Result{
			RequeueAfter: obj.GetRequeueAfter(),
		}, nil
	}

	if !update {
		conditions.Delete(obj, meta.ReconcilingCondition)
		conditions.MarkTrue(obj,
			meta.ReadyCondition,
			meta.SucceededReason,
			fmt.Sprintf("Applied version: %s", version))

		return ctrl.Result{
			RequeueAfter: obj.GetRequeueAfter(),
		}, nil
	}

	ok, err := r.OCMClient.VerifyComponent(ctx, octx, obj, version)
	if err != nil {
		msg := fmt.Sprintf("failed to verify %s with constraint %s with error: %s", obj.Spec.Component, obj.Spec.Version.Semver, err)
		conditions.Delete(obj, meta.ReconcilingCondition)
		conditions.MarkFalse(obj, meta.ReadyCondition, v1alpha1.VerificationFailedReason, msg)
		event.New(r.EventRecorder, obj, eventv1.EventSeverityError, fmt.Sprintf("%s, retrying in %s", err.Error(), obj.GetRequeueAfter()), nil)

		return ctrl.Result{
			RequeueAfter: obj.GetRequeueAfter(),
		}, nil
	}

	if !ok {
		msg := "attempted to verify component, but the digest didn't match"
		conditions.MarkFalse(obj, meta.ReadyCondition, v1alpha1.VerificationFailedReason, msg)
		event.New(r.EventRecorder, obj, eventv1.EventSeverityError, fmt.Sprintf("%s, retrying in %s", msg, obj.GetRequeueAfter()), nil)

		return ctrl.Result{
			RequeueAfter: obj.GetRequeueAfter(),
		}, nil
	}

	// update the result for the defer call to have the latest information
	rresult, err := r.reconcile(ctx, octx, obj, version)
	if err != nil {
		event.New(r.EventRecorder, obj, eventv1.EventSeverityError, fmt.Sprintf("Reconciliation failed: %s, retrying in %s", err.Error(), obj.GetRequeueAfter()), nil)
	}

	return rresult, err
}

func (r *ComponentVersionReconciler) reconcile(ctx context.Context, octx ocm.Context, obj *v1alpha1.ComponentVersion, version string) (ctrl.Result, error) {
	if obj.Generation != obj.Status.ObservedGeneration {
		// don't have to patch here since we patch the object in the outer reconcile call.
		rreconcile.ProgressiveStatus(false, obj, meta.ProgressingReason,
			"processing object: new generation %d -> %d", obj.Status.ObservedGeneration, obj.Generation)
	}

	cv, err := r.OCMClient.GetComponentVersion(ctx, octx, obj, obj.Spec.Component, version)
	if err != nil {
		err = fmt.Errorf("failed to get component version: %w", err)
		conditions.MarkFalse(obj, meta.ReadyCondition, v1alpha1.ComponentVersionInvalidReason, err.Error())
		event.New(r.EventRecorder, obj, eventv1.EventSeverityError, err.Error(), nil)

		return ctrl.Result{}, err
	}

	defer cv.Close()

	// convert ComponentDescriptor to v3alpha1
	dv := &compdesc.DescriptorVersion{}
	cd, err := dv.ConvertFrom(cv.GetDescriptor())
	if err != nil {
		err = fmt.Errorf("failed to convert component descriptor: %w", err)
		conditions.MarkFalse(obj, meta.ReadyCondition, v1alpha1.ConvertComponentDescriptorFailedReason, err.Error())
		return ctrl.Result{}, err
	}

	// setup the component descriptor kubernetes resource
	componentName, err := component.ConstructUniqueName(cd.GetName(), cd.GetVersion(), nil)
	if err != nil {
		err = fmt.Errorf("failed to generate name: %w", err)
		conditions.MarkFalse(obj, meta.ReadyCondition, v1alpha1.NameGenerationFailedReason, err.Error())
		return ctrl.Result{}, err
	}
	descriptor := &v1alpha1.ComponentDescriptor{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: obj.GetNamespace(),
			Name:      componentName,
		},
	}

	//TODO@souleb: pulling instead of doing controllerutil.CreateOrUpdate
	// - can give specific information in eventing
	// - can control creation or update based on a given logic, for drift detection for example.

	// create or update the component descriptor kubernetes resource
	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, descriptor, func() error {
		if descriptor.ObjectMeta.CreationTimestamp.IsZero() {
			if err := controllerutil.SetOwnerReference(obj, descriptor, r.Scheme); err != nil {
				return fmt.Errorf("failed to set owner reference: %w", err)
			}
		}
		spec := v1alpha1.ComponentDescriptorSpec{
			ComponentVersionSpec: cd.(*compdesc.ComponentDescriptor).Spec,
			Version:              cd.GetVersion(),
		}
		descriptor.Spec = spec
		return nil
	})

	if err != nil {
		err = fmt.Errorf("failed to create or update component descriptor: %w", err)
		conditions.MarkFalse(obj, meta.ReadyCondition, v1alpha1.CreateOrUpdateComponentDescriptorFailedReason, err.Error())
		event.New(r.EventRecorder, obj, eventv1.EventSeverityError, err.Error(), nil)
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

	if obj.Spec.References.Expand {
		componentDescriptor.References, err = r.parseReferences(ctx, octx, obj, cv.GetDescriptor().References)
		if err != nil {
			err = fmt.Errorf("failed to parse references: %w", err)
			conditions.MarkFalse(obj, meta.ReadyCondition, v1alpha1.ParseReferencesFailedReason, err.Error())
			event.New(r.EventRecorder, obj, eventv1.EventSeverityError, err.Error(), nil)
			return ctrl.Result{}, err
		}
	}

	obj.Status.ComponentDescriptor = componentDescriptor
	obj.Status.ReconciledVersion = version
	obj.Status.ObservedGeneration = obj.Generation

	conditions.MarkTrue(obj,
		meta.ReadyCondition,
		meta.SucceededReason,
		fmt.Sprintf("Applied version: %s", version))

	conditions.Delete(obj, meta.ReconcilingCondition)

	return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, nil
}

func (r *ComponentVersionReconciler) checkVersion(ctx context.Context, octx ocm.Context, obj *v1alpha1.ComponentVersion) (bool, string, error) {
	log := log.FromContext(ctx).WithName("ocm-component-version-reconcile")

	latest, err := r.OCMClient.GetLatestValidComponentVersion(ctx, octx, obj)
	if err != nil {
		return false, "", fmt.Errorf("failed to get latest component version: %w", err)
	}
	log.V(4).Info("got latest version of component", "version", latest)

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
	log.V(4).Info("current reconciled version is", "reconciled", current.String())

	if latestSemver.Equal(current) || current.GreaterThan(latestSemver) {
		log.V(4).Info("Reconciled version equal to or greater than newest available version", "version", latestSemver)
		return false, "", nil
	}

	event.New(r.EventRecorder, obj, eventv1.EventSeverityInfo, fmt.Sprintf("Version check succeeded, found latest version: %s", latest), nil)
	return true, latest, nil
}

// parseReferences takes a list of references to embedded components and constructs a dependency tree out of them.
// It recursively calls itself, constructing a tree of referenced components. For each referenced component a ComponentDescriptor custom resource will be created.
func (r *ComponentVersionReconciler) parseReferences(ctx context.Context, octx ocm.Context, parent *v1alpha1.ComponentVersion, references ocmdesc.References) ([]v1alpha1.Reference, error) {
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

func (r *ComponentVersionReconciler) constructComponentDescriptorsForReference(ctx context.Context, octx ocm.Context, parent *v1alpha1.ComponentVersion, ref ocmdesc.ComponentReference) (*v1alpha1.Reference, error) {
	// get component version
	rcv, err := r.OCMClient.GetComponentVersion(ctx, octx, parent, ref.ComponentName, ref.Version)
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

	if len(rcv.GetDescriptor().References) > 0 {
		// recursively call parseReference on the embedded references in the new descriptor.
		out, err := r.parseReferences(ctx, nil, parent, rcv.GetDescriptor().References)
		if err != nil {
			return nil, err
		}
		reference.References = out
	}

	return &reference, nil
}

func (r *ComponentVersionReconciler) createComponentDescriptor(ctx context.Context, rcv ocm.ComponentVersionAccess, parent *v1alpha1.ComponentVersion, ref ocmdesc.ComponentReference) (*v1alpha1.ComponentDescriptor, error) {
	// convert ComponentDescriptor to v3alpha1
	dv := &compdesc.DescriptorVersion{}
	cd, err := dv.ConvertFrom(rcv.GetDescriptor())
	if err != nil {
		return nil, fmt.Errorf("failed to convert component descriptor: %w", err)
	}

	log := log.FromContext(ctx)
	// setup the component descriptor kubernetes resource
	componentName, err := component.ConstructUniqueName(ref.ComponentName, ref.Version, ref.GetMeta().ExtraIdentity)
	if err != nil {
		return nil, fmt.Errorf("failed to generate name: %w", err)
	}
	descriptor := &v1alpha1.ComponentDescriptor{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: parent.GetNamespace(),
			Name:      componentName,
		},
		Spec: v1alpha1.ComponentDescriptorSpec{
			ComponentVersionSpec: cd.(*compdesc.ComponentDescriptor).Spec,
			Version:              ref.Version,
		},
	}

	if err := controllerutil.SetOwnerReference(parent, descriptor, r.Scheme); err != nil {
		return nil, fmt.Errorf("failed to set owner reference: %w", err)
	}

	// create or update the component descriptor kubernetes resource
	// we don't need to update it
	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, descriptor, func() error {
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create/update component descriptor: %w", err)
	}
	log.V(4).Info(fmt.Sprintf("%s(ed) descriptor", op), "descriptor", klog.KObj(descriptor))

	return descriptor, nil
}
