/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"strings"

	hash "github.com/mitchellh/hashstructure"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	csdk "github.com/open-component-model/ocm-controllers-sdk"
	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	ocmdesc "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	compdesc "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/ocm.software/v3alpha1"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
)

// ComponentVersionReconciler reconciles a ComponentVersion object
type ComponentVersionReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=delivery.ocm.software,resources=componentversions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=delivery.ocm.software,resources=componentversions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=delivery.ocm.software,resources=componentversions/finalizers,verbs=update

// SetupWithManager sets up the controller with the Manager.
func (r *ComponentVersionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ComponentVersion{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(r)
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

	log.V(4).Info("found component", "component", component)

	session := ocm.NewSession(nil)
	defer session.Close()

	ocmCtx := ocm.ForContext(ctx)

	return r.reconcile(ctx, ocmCtx, session, component)
}

func (r *ComponentVersionReconciler) reconcile(ctx context.Context, ocmCtx ocm.Context, session ocm.Session, obj *v1alpha1.ComponentVersion) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithName("ocm-component-version-reconcile")

	// configure registry credentials
	if err := csdk.ConfigureCredentials(ctx, ocmCtx, r.Client, obj.Spec.Repository.URL, obj.Spec.Repository.SecretRef.Name, obj.Namespace); err != nil {
		log.V(4).Error(err, "failed to find credentials")
		// ignore not found errors for now
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{
				RequeueAfter: obj.GetRequeueAfter(),
			}, fmt.Errorf("failed to configure credentials for component: %w", err)
		}
	}

	// get component version
	cv, err := csdk.GetComponentVersion(ocmCtx, session, obj.Spec.Repository.URL, obj.Spec.Name, obj.Spec.Version)
	if err != nil {
		return ctrl.Result{
			RequeueAfter: obj.GetRequeueAfter(),
		}, fmt.Errorf("failed to get component version: %w", err)
	}

	// convert ComponentDescriptor to v3alpha1
	dv := &compdesc.DescriptorVersion{}
	cd, err := dv.ConvertFrom(cv.GetDescriptor())
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to convret component descriptor: %w", err)
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

	// create or update the component descriptor kubernetes resource
	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, descriptor, func() error {
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

	log.V(4).Info("successfully completed mutation", "operation", op)

	// if references.expand is false then return here
	if !obj.Spec.References.Expand {
		return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, err
	}

	// construct recursive descriptor structure
	// TODO: only do this if expand is true.
	componentDescriptor := v1alpha1.Reference{
		Name:    cd.GetName(),
		Version: cd.GetVersion(),
	}
	componentDescriptor.References, err = r.parseReferences(ctx, ocmCtx, session, obj, cv.GetDescriptor().References)
	if err != nil {
		return ctrl.Result{
			RequeueAfter: obj.GetRequeueAfter(),
		}, fmt.Errorf("failed to get references: %w", err)
	}

	// initialize the patch helper
	patchHelper, err := patch.NewHelper(obj, r.Client)
	if err != nil {
		return ctrl.Result{
			RequeueAfter: obj.GetRequeueAfter(),
		}, fmt.Errorf("failed to create patch helper: %w", err)
	}

	obj.Status.ComponentDescriptor = componentDescriptor

	if err := patchHelper.Patch(ctx, obj); err != nil {
		return ctrl.Result{
			RequeueAfter: obj.GetRequeueAfter(),
		}, fmt.Errorf("failed to patch resource: %w", err)
	}

	log.Info("reconciliation complete")
	return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, nil
}

// parseReferences takes a list of references to embedded components and constructs a dependency tree out of them.
func (r *ComponentVersionReconciler) parseReferences(ctx context.Context, ocmCtx ocm.Context, session ocm.Session, parent *v1alpha1.ComponentVersion, references ocmdesc.References) ([]v1alpha1.Reference, error) {
	log := log.FromContext(ctx)
	result := make([]v1alpha1.Reference, 0)
	for _, ref := range references {
		rcv, err := csdk.GetComponentVersion(ocmCtx, session, parent.Spec.Repository.URL, ref.ComponentName, ref.Version)
		if err != nil {
			return nil, fmt.Errorf("failed to get component version: %w", err)
		}
		// convert ComponentDescriptor to v3alpha1
		dv := &compdesc.DescriptorVersion{}
		cd, err := dv.ConvertFrom(rcv.GetDescriptor())
		if err != nil {
			return nil, fmt.Errorf("failed to convret component descriptor: %w", err)
		}
		// setup the component descriptor kubernetes resource
		componentName, err := r.constructComponentName(rcv.GetName(), rcv.GetVersion(), ref.GetMeta().ExtraIdentity)
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
				Version:              rcv.GetVersion(),
			},
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

		reference := v1alpha1.Reference{
			Name:    rcv.GetName(),
			Version: rcv.GetVersion(),
			ComponentRef: v1alpha1.ComponentRef{
				Name:      descriptor.Name,
				Namespace: descriptor.Namespace,
			},
			ExtraIdentity: ref.ExtraIdentity,
		}

		if len(rcv.GetDescriptor().References) > 0 {
			out, err := r.parseReferences(ctx, ocmCtx, session, parent, rcv.GetDescriptor().References)
			if err != nil {
				return nil, err
			}
			reference.References = out
		}
		result = append(result, reference)
	}
	return result, nil
}

// constructComponentName constructs a unique name from a component name and version.
func (r *ComponentVersionReconciler) constructComponentName(name, version string, identity v1.Identity) (string, error) {
	namingScheme := struct {
		name     string
		version  string
		identity v1.Identity
	}{
		name:     name,
		version:  version,
		identity: identity,
	}
	h, err := hash.Hash(namingScheme, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate hash for name, version, identity: %w", err)
	}
	return fmt.Sprintf("%s-%s-%d", strings.ReplaceAll(name, "/", "-"), version, h), nil
}
