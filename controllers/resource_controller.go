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

	ociclient "github.com/fluxcd/pkg/oci/client"
	"github.com/mandelsoft/vfs/pkg/vfs"
	v1alpha1 "github.com/open-component-model/ocm-controller/api/v1alpha1"
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
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ResourceReconciler reconciles a Resource object
type ResourceReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	OCIRegistryAddr string

	// TODO: Write our own Watch.
	externalTracker external.ObjectTracker
}

//+kubebuilder:rbac:groups=delivery.ocm.software,resources=resources,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=delivery.ocm.software,resources=resources/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=delivery.ocm.software,resources=resources/finalizers,verbs=update

// SetupWithManager sets up the controller with the Manager.
func (r *ResourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	controller, err := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Resource{}).
		Build(r)

	if err != nil {
		return fmt.Errorf("failed setting up with a controller manager: %w", err)
	}

	r.externalTracker = external.ObjectTracker{
		Controller: controller,
	}
	return nil
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *ResourceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithName("resource-controller")

	log.V(4).Info("starting reconcile loop")
	resource := &v1alpha1.Resource{}
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

	return r.reconcile(ctx, resource)
}

func (r *ResourceReconciler) reconcile(ctx context.Context, obj *v1alpha1.Resource) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithName("resource-controller")
	log.V(4).Info("finding component ref", "resource", obj)

	component := &v1alpha1.ComponentVersion{}
	componentKey := types.NamespacedName{
		Name:      obj.Spec.ComponentRef.Name,
		Namespace: obj.Spec.ComponentRef.Namespace,
	}
	if err := r.Get(ctx, componentKey, component); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(4).Info("component not found", "component", obj.Spec.ComponentRef)
			return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, nil
		}

		// Error reading the object - requeue the request.
		return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()},
			fmt.Errorf("failed to get component object: %w", err)
	}

	// TODO: This should be done by the ComponentVersion reconciler.
	session := ocm.NewSession(nil)
	defer session.Close()

	ocmCtx := ocm.ForContext(ctx)
	// configure credentials
	if err := csdk.ConfigureCredentials(ctx, ocmCtx, r.Client, component.Spec.Repository.URL, component.Spec.Repository.SecretRef.Name, component.Namespace); err != nil {
		log.V(4).Error(err, "failed to find credentials")
		// ignore not found errors for now
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{
				RequeueAfter: obj.GetRequeueAfter(),
			}, fmt.Errorf("failed to configure credentials for component: %w", err)
		}
	}

	// get component version
	cv, err := csdk.GetComponentVersion(ocmCtx, session, component.Spec.Repository.URL, component.Spec.Name, component.Spec.Version)
	if err != nil {
		return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, err
	}

	// configure virtual filesystem
	fs, err := csdk.ConfigureTemplateFilesystem(ctx, cv, obj.Spec.Resource)
	if err != nil {
		return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, err
	}
	defer vfs.Cleanup(fs)

	registryURL := r.OCIRegistryAddr
	// add localhost if the only thing defined is a port
	if registryURL[0] == ':' {
		registryURL = "localhost" + registryURL
	}
	snapshot, err := r.transferToObjectStorage(ctx, registryURL, fs, component.Name, obj.Spec.Resource)
	if err != nil {
		return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, err
	}

	// Initialize the patch helper.
	patchHelper, err := patch.NewHelper(obj, r.Client)
	if err != nil {
		return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, err
	}

	obj.Status.Snapshot = snapshot
	obj.Status.Ready = true

	// TODO: Add an ObservedGeneration predicate to avoid an infinite loop of update/reconcile.
	if err := patchHelper.Patch(ctx, obj); err != nil {
		return ctrl.Result{
			RequeueAfter: obj.GetRequeueAfter(),
		}, fmt.Errorf("failed to patch resource and set snaphost value: %w", err)
	}

	return ctrl.Result{}, nil
}

func (r *ResourceReconciler) transferToObjectStorage(ctx context.Context, ociRegistryEndpoint string, virtualFs vfs.FileSystem, repo, resourceName string) (string, error) {
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
