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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api/controllers/external"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	actionv1 "github.com/open-component-model/ocm-controller/api/v1alpha1"
)

// SourceReconciler reconciles a Source object
type SourceReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// TODO: Write our own Watch.
	externalTracker external.ObjectTracker
}

//+kubebuilder:rbac:groups=x-delivery.ocm.software,resources=sources,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=x-delivery.ocm.software,resources=sources/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=x-delivery.ocm.software,resources=sources/finalizers,verbs=update
//+kubebuilder:rbac:groups=x-delivery.ocm.software,resources=*,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *SourceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	source := &actionv1.Source{}
	if err := r.Client.Get(ctx, req.NamespacedName, source); err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return ctrl.Result{}, nil
		}

		// Error reading the object - requeue the request.
		return ctrl.Result{}, fmt.Errorf("failed to get source object: %w", err)
	}

	providerObj, err := Get(ctx, r.Client, &corev1.ObjectReference{
		Kind:       source.Spec.ProviderRef.Kind,
		Name:       source.Spec.ProviderRef.Name,
		APIVersion: source.Spec.ProviderRef.ApiVersion,
	}, source.Namespace)

	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get referenced provider: %w", err)
	}

	// Initialize the patch helper.
	patchHelper, err := patch.NewHelper(source, r.Client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create patch helper: %w", err)
	}

	providerStatus, ok := providerObj.Object["status"]
	if !ok {
		return ctrl.Result{}, fmt.Errorf("failed to find status on referenced provider obj: %+v", *providerObj)
	}
	typedStatus, ok := providerStatus.(map[string]interface{})
	if !ok {
		return ctrl.Result{}, fmt.Errorf("status object of referenced provider is not a map: %+v", providerStatus)
	}
	ready, ok := typedStatus["ready"]
	if !ok {
		return ctrl.Result{}, fmt.Errorf("failed to find ready field on referenced provider obj's status: %+v", typedStatus)
	}
	typedReady, ok := ready.(bool)
	if !ok {
		return ctrl.Result{}, fmt.Errorf("status was not a boolean: %+v", typedReady)
	}

	// we always patch the source object to make sure the status aligns with the provider status.
	source.Status.Ready = typedReady

	// set up snapshot if it exists
	if snapshot, ok := typedStatus["snapshot"]; ok {
		if typedSnapshot, ok := snapshot.(string); ok {
			source.Status.Snapshot = typedSnapshot
		}
	}

	// Patch the external object.
	if err := patchHelper.Patch(ctx, source); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to patch action object: %w", err)
	}

	// Setup watch for the provider referenced object so this `reconcile` is triggered for provider status changes.
	if err := r.externalTracker.Watch(ctrl.Log, providerObj, &handler.EnqueueRequestForOwner{OwnerType: &actionv1.Source{}}); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to set up watch for provider object: %w", err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	controller, err := ctrl.NewControllerManagedBy(mgr).
		For(&actionv1.Source{}).
		Watches(
			&source.Kind{Type: &actionv1.Source{}},
			&handler.EnqueueRequestForObject{}).
		Build(r)

	if err != nil {
		return fmt.Errorf("failed setting up with a controller manager: %w", err)
	}

	r.externalTracker = external.ObjectTracker{
		Controller: controller,
	}
	return nil
}
