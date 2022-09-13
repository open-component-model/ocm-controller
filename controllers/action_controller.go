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

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api/controllers/external"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	actionv1 "github.com/open-component-model/ocm-controller/api/v1alpha1"
)

// ActionReconciler reconciles a Action object
type ActionReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	externalTracker external.ObjectTracker
}

//+kubebuilder:rbac:groups=ocmcontroller.ocm.software,resources=actions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ocmcontroller.ocm.software,resources=actions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ocmcontroller.ocm.software,resources=actions/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *ActionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	action := &actionv1.Action{}
	if err := r.Client.Get(ctx, req.NamespacedName, action); err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return ctrl.Result{}, nil
		}

		// Error reading the object - requeue the request.
		return ctrl.Result{}, fmt.Errorf("failed to get action object: %w", err)
	}

	// Initialize the patch helper.
	patchHelper, err := patch.NewHelper(action, r.Client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create patch helper: %w", err)
	}

	owner, err := r.Get(ctx, &corev1.ObjectReference{
		Kind:       action.Spec.ProviderRef.Kind,
		Name:       action.Spec.ProviderRef.Name,
		APIVersion: action.Spec.ProviderRef.ApiVersion,
	}, action.Namespace)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to find owner for action: %w", err)
	}

	// Set external object ControllerReference to the provider ref.
	if err := controllerutil.SetControllerReference(owner, action, r.Client.Scheme()); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to set owner reference: %w", err)
	}

	// Patch the external object.
	if err := patchHelper.Patch(ctx, action); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to patch action object: %w", err)
	}

	// Ensure we add a watcher to the external object.
	if err := r.externalTracker.Watch(ctrl.Log, owner, &handler.EnqueueRequestForOwner{OwnerType: &actionv1.Action{}}); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to set up watch for parent object: %w", err)
	}

	return ctrl.Result{}, nil
}

// Get uses the client and reference to get an external, unstructured object.
func (r *ActionReconciler) Get(ctx context.Context, ref *corev1.ObjectReference, namespace string) (*unstructured.Unstructured, error) {
	if ref == nil {
		return nil, errors.Errorf("cannot get object - object reference not set")
	}
	obj := new(unstructured.Unstructured)
	obj.SetAPIVersion(ref.APIVersion)
	obj.SetKind(ref.Kind)
	obj.SetName(ref.Name)
	key := client.ObjectKey{Name: obj.GetName(), Namespace: namespace}
	if err := r.Client.Get(ctx, key, obj); err != nil {
		return nil, errors.Wrapf(err, "failed to retrieve %s external object %q/%q", obj.GetKind(), key.Namespace, key.Name)
	}
	return obj, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ActionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	controller, err := ctrl.NewControllerManagedBy(mgr).
		For(&actionv1.Action{}).
		Watches(
			&source.Kind{Type: &actionv1.Action{}},
			&handler.EnqueueRequestForObject{}).
		Build(r)

	if err != nil {
		return errors.Wrap(err, "failed setting up with a controller manager")
	}

	r.externalTracker = external.ObjectTracker{
		Controller: controller,
	}
	return nil
}
