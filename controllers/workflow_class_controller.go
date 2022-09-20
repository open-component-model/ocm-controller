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

	"github.com/go-logr/logr"
	actionv1 "github.com/open-component-model/ocm-controller/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api/controllers/external"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// WorkflowClassReconciler reconciles a Realization object.
type WorkflowClassReconciler struct {
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
func (r *WorkflowClassReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithName("workflow-class-controller")
	log.V(4).Info("starting reconcile loop")

	workflowClass := &actionv1.WorkflowClass{}
	if err := r.Client.Get(ctx, req.NamespacedName, workflowClass); err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return ctrl.Result{}, nil
		}

		// Error reading the object - requeue the request.
		return ctrl.Result{}, fmt.Errorf("failed to get action object: %w", err)
	}
	log.V(4).Info("reconciling workflow class", "workflow-class", workflowClass)

	for _, workflow := range workflowClass.Spec.Workflows {
		obj, err := r.createObject(ctx, log, workflow, workflowClass)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to create object with name '%s': %w", workflow.Name, err)
		}
		log.V(4).Info("created object", "obj", obj)
	}

	return ctrl.Result{}, nil
}

func (r *WorkflowClassReconciler) createObject(ctx context.Context, log logr.Logger, workflow actionv1.ClassWorkflow, workflowClass *actionv1.WorkflowClass) (client.Object, error) {
	log = log.WithValues("workflow", workflowClass)
	stage, ok := workflowClass.Spec.Stages[workflow.Name]
	if !ok {
		return nil, fmt.Errorf("failed to find referenced workflow item with name '%s'", workflow.Name)
	}
	provider, err := r.createProviderObject(ctx, log, stage.Provider, workflow.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to create provider object: %w", err)
	}
	var obj client.Object
	switch stage.Type {
	case "Action":
		obj = &actionv1.Action{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Action",
				APIVersion: actionv1.GroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      workflow.Name,
				Namespace: workflowClass.Namespace,
			},
			Spec: actionv1.ActionSpec{
				ComponentRef: actionv1.ComponentRef{
					Namespace: workflowClass.Namespace,
					Name:      "get this from the Workflow componentRef",
				},
				ProviderRef: actionv1.ProviderRef{
					ApiVersion: stage.Provider.APIVersion,
					Kind:       stage.Provider.Kind,
					Name:       provider.GetName(),
				},
			},
		}
		if workflow.Input != "" {
			dependentStage, ok := workflowClass.Spec.Stages[workflow.Input]
			if !ok {
				return nil, fmt.Errorf("failed to find referenced workflow input with name '%s'", workflow.Input)
			}
			obj.(*actionv1.Action).Spec.SourceRef = actionv1.SourceRef{
				Name:       workflow.Input,
				Kind:       dependentStage.Type,
				ApiVersion: actionv1.GroupVersion.String(),
			}
		}
	case "Source":
		obj = &actionv1.Source{}
	}

	if err := r.Client.Create(ctx, obj); err != nil {
		log.Error(err, "failed to create the appropriate resource", "obj", obj)
		return nil, fmt.Errorf("failed to create appropriate resource: %w", err)
	}
	return obj, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *WorkflowClassReconciler) SetupWithManager(mgr ctrl.Manager) error {
	controller, err := ctrl.NewControllerManagedBy(mgr).
		For(&actionv1.WorkflowClass{}).
		Build(r)

	if err != nil {
		return fmt.Errorf("failed setting up with a controller manager: %w", err)
	}

	r.externalTracker = external.ObjectTracker{
		Controller: controller,
	}
	return nil
}

func (r *WorkflowClassReconciler) createProviderObject(ctx context.Context, log logr.Logger, provider actionv1.Provider, workflow string) (client.Object, error) {
	name := fmt.Sprintf("workflow-%s", workflow)
	obj := new(unstructured.Unstructured)
	obj.SetAPIVersion(provider.APIVersion)
	obj.SetKind(provider.Kind)
	obj.SetName(name)
	if err := r.Client.Create(ctx, obj); err != nil {
		log.Error(err, "failed to create the provider object", "obj", obj)
		return nil, fmt.Errorf("failed to create provider object: %w", err)
	}
	return obj, nil
}
