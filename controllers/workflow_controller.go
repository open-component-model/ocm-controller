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
	"time"

	actionv1 "github.com/open-component-model/ocm-controller/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api/controllers/external"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// RealizationReconciler reconciles a Realization object.
type RealizationReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// TODO: Write our own Watch.
	externalTracker external.ObjectTracker
}

//+kubebuilder:rbac:groups=delivery.ocm.software,resources=actions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=delivery.ocm.software,resources=actions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=delivery.ocm.software,resources=actions/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *RealizationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithName("action-controller")
	log.V(4).Info("starting reconcile loop")

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
	log.V(4).Info("reconciling action", "action", action)
	providerObj, err := Get(ctx, r.Client, &corev1.ObjectReference{
		Kind:       action.Spec.ProviderRef.Kind,
		Name:       action.Spec.ProviderRef.Name,
		APIVersion: action.Spec.ProviderRef.ApiVersion,
	}, action.Namespace)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to find owner for action: %w", err)
	}
	log.V(4).Info("found provider object", "provider", providerObj)
	// Set external object ControllerReference to the provider ref.
	if err := controllerutil.SetControllerReference(action, providerObj, r.Client.Scheme()); err != nil {
		return ctrl.Result{
			RequeueAfter: 1 * time.Minute,
		}, fmt.Errorf("failed to set owner reference: %w", err)
	}

	log.V(4).Info("set up controller reference")
	// Initialize the patch helper.
	patchHelper, err := patch.NewHelper(providerObj, r.Client)
	if err != nil {
		return ctrl.Result{
			RequeueAfter: 1 * time.Minute,
		}, fmt.Errorf("failed to create patch helper: %w", err)
	}

	// Patch the external object.
	if err := patchHelper.Patch(ctx, providerObj); err != nil {
		return ctrl.Result{
			RequeueAfter: 1 * time.Minute,
		}, fmt.Errorf("failed to patch provider object: %w", err)
	}
	log.V(4).Info("path successful")
	// Ensure we add a watcher to the external object.
	if err := r.externalTracker.Watch(ctrl.Log, providerObj, &handler.EnqueueRequestForOwner{OwnerType: &actionv1.Action{}}); err != nil {
		return ctrl.Result{
			RequeueAfter: 1 * time.Minute,
		}, fmt.Errorf("failed to set up watch for parent object: %w", err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RealizationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	controller, err := ctrl.NewControllerManagedBy(mgr).
		For(&actionv1.Action{}).
		Build(r)

	if err != nil {
		return fmt.Errorf("failed setting up with a controller manager: %w", err)
	}

	r.externalTracker = external.ObjectTracker{
		Controller: controller,
	}
	return nil
}
