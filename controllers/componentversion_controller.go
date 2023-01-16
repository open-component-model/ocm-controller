// Copyright 2022.
// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"fmt"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/patch"
	hash "github.com/mitchellh/hashstructure"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	ocmdesc "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc"
	v1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	compdesc "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/ocm.software/v3alpha1"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	ocmclient "github.com/open-component-model/ocm-controller/pkg/ocm"
)

// ComponentVersionReconciler reconciles a ComponentVersion object
type ComponentVersionReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	OCMClient ocmclient.FetchVerifier
}

//+kubebuilder:rbac:groups=delivery.ocm.software,resources=componentversions;componentdescriptors,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=delivery.ocm.software,resources=componentversions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=delivery.ocm.software,resources=componentversions/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=services;pods,verbs=get;create;update;patch;delete
//+kubebuilder:rbac:groups="apps",resources=deployments,verbs=get;create;update;patch;delete

// +kubebuilder:rbac:groups="",resources=secrets;serviceaccounts,verbs=get;list;watch

// SetupWithManager sets up the controller with the Manager.
func (r *ComponentVersionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		//TODO@souleb: add delete predicate,
		// I believe we want to clean up the component descriptor and resources on delete.
		// We need a finalizer for that
		For(&v1alpha1.ComponentVersion{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(r)

	//TODO@souleb: add watch for component descriptors
	// We want to be notified if a component descriptor changes, maybe by a human actor.
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ComponentVersionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithName("ocm-component-version-reconcile")

	log.Info("starting ocm component loop")

	component := &v1alpha1.ComponentVersion{}
	if err := r.Client.Get(ctx, req.NamespacedName, component); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get component object: %w", err)
	}

	//TODO@souleb: reduce logging verbosity, use events instead. This will be easier to use logs
	// for debugging and events for monitoring
	log.V(4).Info("found component", "component", component)

	// reconcile the version before calling reconcile func
	update, version, err := r.checkVersion(ctx, component)
	if err != nil {
		return ctrl.Result{
			RequeueAfter: component.GetRequeueAfter(),
		}, fmt.Errorf("failed to check version: %w", err)
	}

	if !update {
		log.V(4).Info("no new version which satisfies the semver constraint detected")
		return ctrl.Result{
			RequeueAfter: component.GetRequeueAfter(),
		}, nil
	}

	log.Info("running verification of component")
	ok, err := r.OCMClient.VerifyComponent(ctx, component, version)
	if err != nil {
		return ctrl.Result{
			RequeueAfter: component.GetRequeueAfter(),
		}, fmt.Errorf("failed to verify component: %w", err)
	}

	if !ok {
		return ctrl.Result{
			RequeueAfter: component.GetRequeueAfter(),
		}, fmt.Errorf("attempted to verify component, but the digest didn't match")
	}

	return r.reconcile(ctx, component, version)
}

func (r *ComponentVersionReconciler) checkVersion(ctx context.Context, obj *v1alpha1.ComponentVersion) (bool, string, error) {
	log := log.FromContext(ctx).WithName("ocm-component-version-reconcile")

	// get given semver constraint
	constraint, err := semver.NewConstraint(obj.Spec.Version.Semver)
	if err != nil {
		return false, "", fmt.Errorf("failed to parse given semver constraint: %w", err)
	}

	// get current reconciled version
	reconciledVersion := "0.0.0"
	if obj.Status.ReconciledVersion != "" {
		reconciledVersion = obj.Status.ReconciledVersion
	}
	current, err := semver.NewVersion(reconciledVersion)
	if err != nil {
		return false, "", fmt.Errorf("failed to parse reconciled version: %w", err)
	}
	log.V(4).Info("current reconciled version is", "reconciled", current.String())

	// get latest available component version
	latest, err := r.OCMClient.GetLatestComponentVersion(ctx, obj)
	if err != nil {
		return false, "", fmt.Errorf("failed to get latest component version: %w", err)
	}
	log.V(4).Info("got newest version from component", "version", latest)

	latestSemver, err := semver.NewVersion(latest)
	if err != nil {
		return false, "", fmt.Errorf("failed to parse latest version: %w", err)
	}

	// compare given constraint and latest available version
	valid, errs := constraint.Validate(latestSemver)
	if !valid || len(errs) > 0 {
		log.Info("version constraint check failed with the following problems", "errors", errs, "version", current)
		return false, "", nil
	}

	return true, latest, nil
}

func (r *ComponentVersionReconciler) reconcile(ctx context.Context, obj *v1alpha1.ComponentVersion, version string) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithName("ocm-component-version-reconcile")

	// get component version
	// TODO@souleb: resolve all referenced dependencies and pass them to the function
	// this will allow us to return early if the dependencies are not ready, and set error
	// conditions on the component version.
	// We can the pass the needed dependencies to the function as Option
	cv, err := r.OCMClient.GetComponentVersion(ctx, obj, obj.Spec.Component, version)
	if err != nil {
		return ctrl.Result{
			RequeueAfter: obj.GetRequeueAfter(),
		}, fmt.Errorf("failed to get component version: %w", err)
	}
	defer cv.Close()

	// convert ComponentDescriptor to v3alpha1
	dv := &compdesc.DescriptorVersion{}
	cd, err := dv.ConvertFrom(cv.GetDescriptor())
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to convert component descriptor: %w", err)
	}

	// setup the component descriptor kubernetes resource
	componentName, err := r.constructComponentName(cd.GetName(), cd.GetVersion(), nil)
	if err != nil {
		return ctrl.Result{
			RequeueAfter: obj.GetRequeueAfter(),
		}, fmt.Errorf("failed to generate name: %w", err)
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
		return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()},
			fmt.Errorf("failed to create or update component descriptor: %w", err)
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
		componentDescriptor.References, err = r.parseReferences(ctx, obj, cv.GetDescriptor().References)
		if err != nil {
			return ctrl.Result{
				RequeueAfter: obj.GetRequeueAfter(),
			}, fmt.Errorf("failed to get references: %w", err)
		}
	}

	// initialize the patch helper
	patchHelper, err := patch.NewHelper(obj, r.Client)
	if err != nil {
		return ctrl.Result{
			RequeueAfter: obj.GetRequeueAfter(),
		}, fmt.Errorf("failed to create patch helper: %w", err)
	}

	obj.Status.ComponentDescriptor = componentDescriptor
	obj.Status.ReconciledVersion = version

	if err := patchHelper.Patch(ctx, obj); err != nil {
		return ctrl.Result{
			RequeueAfter: obj.GetRequeueAfter(),
		}, fmt.Errorf("failed to patch resource: %w", err)
	}

	log.Info("reconciliation complete")
	return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, nil
}

// parseReferences takes a list of references to embedded components and constructs a dependency tree out of them.
// It recursively calls itself, constructing a tree of referenced components. For each referenced component a ComponentDescriptor custom resource will be created.
func (r *ComponentVersionReconciler) parseReferences(ctx context.Context, parent *v1alpha1.ComponentVersion, references ocmdesc.References) ([]v1alpha1.Reference, error) {
	result := make([]v1alpha1.Reference, 0)
	for _, ref := range references {
		reference, err := r.constructComponentDescriptorsForReference(ctx, parent, ref)
		if err != nil {
			return nil, fmt.Errorf("failed to construct component descriptor: %w", err)
		}
		result = append(result, *reference)
	}
	return result, nil
}

func (r *ComponentVersionReconciler) constructComponentDescriptorsForReference(ctx context.Context, parent *v1alpha1.ComponentVersion, ref ocmdesc.ComponentReference) (*v1alpha1.Reference, error) {
	// get component version
	rcv, err := r.OCMClient.GetComponentVersion(ctx, parent, ref.ComponentName, ref.Version)
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
		out, err := r.parseReferences(ctx, parent, rcv.GetDescriptor().References)
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
	componentName, err := r.constructComponentName(ref.ComponentName, ref.Version, ref.GetMeta().ExtraIdentity)
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

// constructComponentName constructs a unique name from a component name and version.
func (r *ComponentVersionReconciler) constructComponentName(name, version string, identity v1.Identity) (string, error) {
	namingScheme := struct {
		ComponentName string
		Version       string
		Identity      v1.Identity
	}{
		ComponentName: name,
		Version:       version,
		Identity:      identity,
	}
	h, err := hash.Hash(namingScheme, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate hash for name, version, identity: %w", err)
	}
	return fmt.Sprintf("%s-%s-%d", strings.ReplaceAll(name, "/", "-"), version, h), nil
}
