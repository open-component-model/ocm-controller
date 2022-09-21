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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api/controllers/external"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"

	actionv1 "github.com/open-component-model/ocm-controller/api/v1alpha1"
	csdk "github.com/open-component-model/ocm-controllers-sdk"
	"github.com/open-component-model/ocm/pkg/contexts/ocm"
)

// WorkflowReconciler reconciles a Realization object.
type WorkflowReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// TODO: Write our own Watch.
	externalTracker external.ObjectTracker
}

//+kubebuilder:rbac:groups=delivery.ocm.software,resources=workflows,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=delivery.ocm.software,resources=workflows/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=delivery.ocm.software,resources=workflows/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *WorkflowReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithName("workflow-controller")
	log.V(4).Info("starting reconcile loop")

	workflow := &actionv1.Workflow{}
	if err := r.Client.Get(ctx, req.NamespacedName, workflow); err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return ctrl.Result{}, nil
		}

		// Error reading the object - requeue the request.
		return ctrl.Result{}, fmt.Errorf("failed to get action object: %w", err)
	}
	log.V(4).Info("reconciling workflow", "workflow", workflow)

	// Assuming OCMComponent already created the OCM object... Or maybe just downloaded it?
	// We would have to somehow find that object. What would be its name? The component would live as name component name
	// and then the namespace, right?
	component := &actionv1.OCMComponentVersion{}
	if err := r.Client.Get(ctx, types.NamespacedName{
		Namespace: workflow.Spec.ComponentRef.Name,
		Name:      workflow.Spec.ComponentRef.Namespace,
	}, component); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		// Error reading the object - requeue the request.
		return ctrl.Result{}, fmt.Errorf("failed to get component object: %w", err)
	}
	log.V(4).Info("found component for component ref in workflow", "component", component)

	// Now, I'm going to download and parse the component descriptor, but in the future, the OCMComponent should have
	// applied it as a CRD and I would just fetch that object.

	session := ocm.NewSession(nil)
	defer session.Close()

	ocmCtx := ocm.ForContext(ctx)
	// configure credentials
	if err := csdk.ConfigureCredentials(ctx, ocmCtx, r.Client, component.Spec.Repository.URL, component.Spec.Repository.SecretRef.Name, component.Namespace); err != nil {
		log.V(4).Error(err, "failed to find credentials")
		// ignore not found errors for now
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{
				RequeueAfter: component.Spec.Interval,
			}, fmt.Errorf("failed to configure credentials for component: %w", err)
		}
	}

	// get component version
	cv, err := csdk.GetComponentVersion(ocmCtx, session, component.Spec.Repository.URL, component.Spec.Name, component.Spec.Version)
	if err != nil {
		return ctrl.Result{
			RequeueAfter: component.Spec.Interval,
		}, fmt.Errorf("failed to get component version: %w", err)
	}

	_, b, err := csdk.GetResourceForComponentVersion(cv, workflow.Spec.ClassResource.Name)
	if err != nil {
		return ctrl.Result{
			RequeueAfter: component.Spec.Interval,
		}, fmt.Errorf("failed to get resource for component version: %w", err)
	}

	workflowClass := &actionv1.WorkflowClass{}
	if err := yaml.Unmarshal(b.Bytes(), workflowClass); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to unmarshall template file content into WorkflowClass: %w", err)
	}

	log.V(4).Info("reconciling workflow class", "workflow-class", workflowClass)

	for _, w := range workflowClass.Spec.Workflow {
		obj, err := r.createObject(ctx, log, w, workflowClass, workflow)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to create object with name '%s': %w", w.Name, err)
		}
		log.V(4).Info("created object", "obj", obj)
	}

	return ctrl.Result{}, nil
}

func (r *WorkflowReconciler) createObject(ctx context.Context, log logr.Logger, workflow actionv1.WorkflowItem, workflowClass *actionv1.WorkflowClass, w *actionv1.Workflow) (client.Object, error) {
	log = log.WithValues("workflow", workflow)
	stage, ok := workflowClass.Spec.Stages[workflow.Name]
	if !ok {
		return nil, fmt.Errorf("failed to find referenced workflow item with name '%s'", workflow.Name)
	}
	provider, err := r.createProviderObject(ctx, log, stage.Provider, workflowClass.Name, workflow.Name)
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
		obj = &actionv1.Source{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Source",
				APIVersion: actionv1.GroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      workflow.Name,
				Namespace: workflowClass.Namespace,
			},
			Spec: actionv1.SourceSpec{
				ComponentRef: w.Spec.ComponentRef,
				ProviderRef: actionv1.ProviderRef{
					ApiVersion: stage.Provider.APIVersion,
					Kind:       stage.Provider.Kind,
					Name:       provider.GetName(),
				},
			},
		}
	}

	if err := r.Client.Create(ctx, obj); err != nil {
		log.Error(err, "failed to create the appropriate resource", "obj", obj)
		return nil, fmt.Errorf("failed to create appropriate resource: %w", err)
	}
	return obj, nil
}

func (r *WorkflowReconciler) createProviderObject(ctx context.Context, log logr.Logger, provider actionv1.Provider, workflowName, stageName string) (client.Object, error) {
	name := fmt.Sprintf("%s-%s", workflowName, stageName)
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

// SetupWithManager sets up the controller with the Manager.
func (r *WorkflowReconciler) SetupWithManager(mgr ctrl.Manager) error {
	controller, err := ctrl.NewControllerManagedBy(mgr).
		For(&actionv1.Workflow{}).
		Build(r)

	if err != nil {
		return fmt.Errorf("failed setting up with a controller manager: %w", err)
	}

	r.externalTracker = external.ObjectTracker{
		Controller: controller,
	}
	return nil
}
