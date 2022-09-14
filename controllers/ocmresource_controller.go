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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api/controllers/external"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	actionv1 "github.com/open-component-model/ocm-controller/api/v1alpha1"
)

// OCMResourceReconciler reconciles a OCMResource object
type OCMResourceReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// TODO: Write our own Watch.
	externalTracker external.ObjectTracker
}

//+kubebuilder:rbac:groups=x-delivery.ocm.software,resources=ocmresources,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=x-delivery.ocm.software,resources=ocmresources/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=x-delivery.ocm.software,resources=ocmresources/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *OCMResourceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithName("ocmresource-controller")

	log.V(4).Info("starting reconcile loop")
	resource := &actionv1.OCMResource{}
	if err := r.Client.Get(ctx, req.NamespacedName, resource); err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return ctrl.Result{}, nil
		}

		// Error reading the object - requeue the request.
		return ctrl.Result{}, fmt.Errorf("failed to get resource object: %w", err)
	}
	log.V(4).Info("found resource", "resource", resource)

	// Set up a watch on the parent Source
	parent, err := r.getParentSource(ctx, resource)
	if err != nil {
		log.Info("parent source for ocm resource is not yet available... requeuing...")
		return ctrl.Result{
			RequeueAfter: 1 * time.Minute,
		}, nil
	}

	log.V(4).Info("found parent source", "parent", parent)
	// Watch the parent for changes in componentRef?
	// get that component and do what with it?
	if err := r.externalTracker.Watch(ctrl.Log, parent, &handler.EnqueueRequestForOwner{OwnerType: &actionv1.Source{}}); err != nil {
		return ctrl.Result{
			RequeueAfter: 1 * time.Minute,
		}, fmt.Errorf("failed to set up watch for source object: %w", err)
	}

	log.V(4).Info("finding component ref", "resource", resource)
	component := &actionv1.OCMComponent{}
	if err := r.Client.Get(ctx, types.NamespacedName{
		Name:      parent.Spec.ComponentRef.Name,
		Namespace: parent.Spec.ComponentRef.Namespace,
	}, component); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(4).Info("component not found", "component", parent.Spec.ComponentRef)
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return ctrl.Result{}, nil
		}

		// Error reading the object - requeue the request.
		return ctrl.Result{
			RequeueAfter: 1 * time.Minute,
		}, fmt.Errorf("failed to get component object: %w", err)
	}

	log.V(4).Info("found component object", "component", component)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OCMResourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	controller, err := ctrl.NewControllerManagedBy(mgr).
		For(&actionv1.OCMResource{}).
		Watches(
			&source.Kind{Type: &actionv1.OCMResource{}},
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

func (r *OCMResourceReconciler) getParentSource(ctx context.Context, obj *actionv1.OCMResource) (*actionv1.Source, error) {
	for _, ref := range obj.OwnerReferences {
		if ref.Kind != "Source" {
			continue
		}

		gv, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			return nil, err
		}

		if gv.Group != actionv1.GroupVersion.Group {
			continue
		}

		source := &actionv1.Source{}
		key := client.ObjectKey{
			Namespace: obj.Namespace,
			Name:      ref.Name,
		}

		if err := r.Client.Get(ctx, key, source); err != nil {
			return nil, fmt.Errorf("failed to get parent Source: %w", err)
		}

		return source, nil
	}

	// return not found error ?
	return nil, fmt.Errorf("parent not found")
}
