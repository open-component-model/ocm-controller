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
	"os"
	"path/filepath"
	"time"

	ociclient "github.com/fluxcd/pkg/oci/client"
	"github.com/mandelsoft/vfs/pkg/vfs"
	actionv1 "github.com/open-component-model/ocm-controller/api/v1alpha1"
	csdk "github.com/open-component-model/ocm-controllers-sdk"
	"github.com/open-component-model/ocm-controllers-sdk/oci"
	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api/controllers/external"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// OCMResourceReconciler reconciles a OCMResource object
type OCMResourceReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	OCIRegistryAddr string

	// TODO: Write our own Watch.
	externalTracker external.ObjectTracker
}

//+kubebuilder:rbac:groups=delivery.ocm.software,resources=ocmresources,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=delivery.ocm.software,resources=ocmresources/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=delivery.ocm.software,resources=ocmresources/finalizers,verbs=update

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

	if resource.Status.Ready {
		// TODO: I wonder if this is a good idea. This means that this resource will never be updated / reprocessed.
		log.V(4).Info("skipping resource as it has already been reconciled", "resource", resource)
		return ctrl.Result{}, nil
	}

	// Set up a watch on the parent Source
	parent := &actionv1.Source{}
	if err := csdk.GetParentObject(ctx, r.Client, "Source", actionv1.GroupVersion.Group, resource, parent); err != nil {
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
			return ctrl.Result{}, nil
		}

		// Error reading the object - requeue the request.
		return ctrl.Result{
			RequeueAfter: 1 * time.Minute,
		}, fmt.Errorf("failed to get component object: %w", err)
	}

	// TODO: Would gather the ComponentDescritor object from the cluster that OCMComponent controller applied.
	// Location to the component descriptor.

	log.V(4).Info("found component object", "component", component)

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
		}, err
	}

	// configure virtual filesystem
	fs, err := csdk.ConfigureTemplateFilesystem(ctx, cv, resource.Spec.Resource)
	if err != nil {
		return ctrl.Result{
			RequeueAfter: component.Spec.Interval,
		}, err
	}
	defer vfs.Cleanup(fs)

	registryURL := r.OCIRegistryAddr
	// add localhost if the only thing defined is a port
	if registryURL[0] == ':' {
		registryURL = "localhost" + registryURL
	}
	snapshot, err := r.transferToObjectStorage(ctx, registryURL, fs, component.Name, resource.Spec.Resource)
	if err != nil {
		return ctrl.Result{
			RequeueAfter: component.Spec.Interval,
		}, err
	}

	// Initialize the patch helper.
	patchHelper, err := patch.NewHelper(resource, r.Client)
	if err != nil {
		return ctrl.Result{
			RequeueAfter: component.Spec.Interval,
		}, fmt.Errorf("failed to create patch helper: %w", err)
	}

	resource.Status.Snapshot = snapshot
	resource.Status.Ready = true

	// TODO: Add an ObservedGeneration predicate to avoid an infinite loop of update/reconcile.
	if err := patchHelper.Patch(ctx, resource); err != nil {
		return ctrl.Result{
			RequeueAfter: component.Spec.Interval,
		}, fmt.Errorf("failed to patch resource and set snaphost value: %w", err)
	}

	return ctrl.Result{}, nil
}

func (r *OCMResourceReconciler) transferToObjectStorage(ctx context.Context, ociRegistryEndpoint string, virtualFs vfs.FileSystem, repo, resourceName string) (string, error) {
	log := log.FromContext(ctx)
	rootDir := "/"

	fi, err := virtualFs.Stat(rootDir)
	if err != nil {
		return "", err
	}

	sourceDir := filepath.Join(os.TempDir(), fi.Name())
	artifactPath := filepath.Join(os.TempDir(), fi.Name()+".tar.gz")

	// We have the source dir, just tar and upload it to the registry.
	metadata := ociclient.Metadata{
		Source:   "github.com/open-component-model/ocm-controller",
		Revision: "rev",
	}

	snapshotName := csdk.GetSnapshotName(repo, resourceName)
	taggedURL := fmt.Sprintf("%s/%s", ociRegistryEndpoint, snapshotName)
	log.V(4).Info("pushing joined url", "url", taggedURL)
	pusher := oci.NewClient(taggedURL)

	if err := pusher.Push(ctx, artifactPath, sourceDir, metadata); err != nil {
		return "", fmt.Errorf("failed to push artifact: %w", err)
	}
	log.V(4).Info("successfully uploaded artifact to location", "location", artifactPath, "sourcedir", sourceDir)

	return snapshotName, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OCMResourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	controller, err := ctrl.NewControllerManagedBy(mgr).
		For(&actionv1.OCMResource{}).
		Build(r)

	if err != nil {
		return fmt.Errorf("failed setting up with a controller manager: %w", err)
	}

	r.externalTracker = external.ObjectTracker{
		Controller: controller,
	}
	return nil
}
