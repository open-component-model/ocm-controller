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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "github.com/open-component-model/ocm-controller/api/v1alpha1"
	csdk "github.com/open-component-model/ocm-controllers-sdk"

	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	compdesc "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/ocm.software/v3alpha1"
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
		For(&v1alpha1.ComponentVersion{}).
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

	return r.reconcile(ctx, component)
}

func (r *ComponentVersionReconciler) reconcile(ctx context.Context, obj *v1alpha1.ComponentVersion) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithName("ocm-component-version-reconcile")

	session := ocm.NewSession(nil)
	defer session.Close()

	ocmCtx := ocm.ForContext(ctx)

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

	// convert ComponentDescriptor to v32alpha1
	dv := &compdesc.DescriptorVersion{}
	cd, err := dv.ConvertFrom(cv.GetDescriptor())
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to convret component descriptor: %w", err)
	}

	// setup the component descriptor kubernetes resource
	descriptor := &v1alpha1.ComponentDescriptor{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: obj.GetNamespace(),
			Name:      strings.ReplaceAll(cv.GetName(), "/", "."),
		},
	}

	// create or update the component descriptor kubernetes resource
	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, descriptor, func() error {
		if descriptor.ObjectMeta.CreationTimestamp.IsZero() {
			controllerutil.SetOwnerReference(obj, descriptor, r.Scheme)
		}
		descriptor.Spec = cd.(*compdesc.ComponentDescriptor).Spec
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

	expandedRefs := make(map[string]string)
	// iterate referenced component descriptors
	for _, ref := range cv.GetDescriptor().References {
		rcv, err := csdk.GetComponentVersion(ocmCtx, session, obj.Spec.Repository.URL, ref.ComponentName, ref.Version)
		if err != nil {
			return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()},
				fmt.Errorf("failed to get component version: %w", err)
		}

		dv := &compdesc.DescriptorVersion{}
		rcd, err := dv.ConvertFrom(rcv.GetDescriptor())
		if err != nil {
			return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()},
				fmt.Errorf("failed to convert component descriptor: %w", err)
		}

		rdescriptor := &v1alpha1.ComponentDescriptor{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: obj.GetNamespace(),
				Name:      strings.ReplaceAll(rcv.GetName(), "/", "."),
			},
		}

		op, err := controllerutil.CreateOrUpdate(ctx, r.Client, rdescriptor, func() error {
			if descriptor.ObjectMeta.CreationTimestamp.IsZero() {
				controllerutil.SetOwnerReference(obj, rdescriptor, r.Scheme)
			}
			descriptor.Spec = rcd.(*compdesc.ComponentDescriptor).Spec
			return nil
		})

		if err != nil {
			return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()},
				fmt.Errorf("failed to create or update component descriptor: %w", err)
		}

		expandedRefs[ref.Name] = rdescriptor.GetNamespace() + "/" + rdescriptor.GetName()

		log.V(4).Info("successfully completed mutation", "operation", op)
	}

	// Initialize the patch helper.
	patchHelper, err := patch.NewHelper(obj, r.Client)
	if err != nil {
		return ctrl.Result{
			RequeueAfter: obj.GetRequeueAfter(),
		}, fmt.Errorf("failed to create patch helper: %w", err)
	}

	// write component descriptor details to status
	if obj.Status.ComponentDescriptors == nil {
		obj.Status.ComponentDescriptors = make(map[string]string)
	}

	obj.Status.ComponentDescriptors["root"] = descriptor.GetNamespace() + "/" + descriptor.GetName()
	for k, v := range expandedRefs {
		obj.Status.ComponentDescriptors[k] = v
	}

	if err := patchHelper.Patch(ctx, obj); err != nil {
		return ctrl.Result{
			RequeueAfter: obj.GetRequeueAfter(),
		}, fmt.Errorf("failed to patch resource: %w", err)
	}

	log.Info("reconciliation complete")
	return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, nil
}
