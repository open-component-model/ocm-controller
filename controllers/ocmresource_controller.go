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
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/containers/image/v5/pkg/compression"
	ociclient "github.com/fluxcd/pkg/oci/client"
	"github.com/mandelsoft/vfs/pkg/osfs"
	"github.com/mandelsoft/vfs/pkg/vfs"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/cpi"
	"github.com/open-component-model/ocm/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api/controllers/external"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/open-component-model/ocm/pkg/common"
	"github.com/open-component-model/ocm/pkg/contexts/credentials"
	"github.com/open-component-model/ocm/pkg/contexts/oci/repositories/ocireg"
	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	ocmmeta "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/repositories/genericocireg"

	actionv1 "github.com/open-component-model/ocm-controller/api/v1alpha1"
	registry "github.com/open-component-model/ocm-controller/pkg/registry"
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

	if resource.Status.Ready {
		// TODO: I wonder if this is a good idea. This means that this resource will never be updated / reprocessed.
		log.V(4).Info("skipping resource as it has already been reconciled", "resource", resource)
		return ctrl.Result{}, nil
	}

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

	session := ocm.NewSession(nil)
	defer session.Close()

	ocmCtx := ocm.ForContext(ctx)
	// configure credentials
	if err := r.configureCredentials(ctx, ocmCtx, component); err != nil {
		log.V(4).Error(err, "failed to find credentials")
		// ignore not found errors for now
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{
				RequeueAfter: component.Spec.Interval,
			}, fmt.Errorf("failed to configure credentials for component: %w", err)
		}
	}
	// get component version
	cv, err := r.getComponentVersion(ocmCtx, session, component)
	if err != nil {
		return ctrl.Result{
			RequeueAfter: component.Spec.Interval,
		}, err
	}

	// configure virtual filesystem
	fs, err := r.configureTemplateFilesystem(ctx, cv, resource.Spec.Resource)
	if err != nil {
		return ctrl.Result{
			RequeueAfter: component.Spec.Interval,
		}, err
	}
	defer vfs.Cleanup(fs)

	// put the stuff into the oci registry and return the snapshot? ( patch the snapshot and status to Ready ).
	snapshot, err := r.transferToObjectStorage(ctx, "localhost:5000", fs, component.Name, resource.Spec.Resource)
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

	if err := patchHelper.Patch(ctx, resource); err != nil {
		return ctrl.Result{
			RequeueAfter: component.Spec.Interval,
		}, fmt.Errorf("failed to patch resource and set snaphost value: %w", err)
	}

	return ctrl.Result{}, nil
}

func (r *OCMResourceReconciler) configureCredentials(ctx context.Context, ocmCtx ocm.Context, component *actionv1.OCMComponent) error {
	// create the consumer id for credentials
	consumerID, err := getConsumerIdentityForRepository(component.Spec.Repository)
	if err != nil {
		return err
	}

	// fetch the credentials for the component storage
	creds, err := r.getCredentialsForRepository(ctx, component.GetNamespace(), component.Spec.Repository)
	if err != nil {
		return err
	}

	// TODO: set credentials should return an error
	ocmCtx.CredentialsContext().SetCredentialsForConsumer(consumerID, creds)

	return nil
}

func (r *OCMResourceReconciler) getCredentialsForRepository(ctx context.Context, namespace string, repo actionv1.Repository) (credentials.Credentials, error) {
	var secret corev1.Secret
	secretKey := client.ObjectKey{
		Namespace: namespace,
		Name:      repo.SecretRef.Name,
	}
	if err := r.Get(ctx, secretKey, &secret); err != nil {
		return nil, err
	}

	props := make(common.Properties)
	for key, value := range secret.Data {
		props.SetNonEmptyValue(key, string(value))
	}

	return credentials.NewCredentials(props), nil
}

func (r *OCMResourceReconciler) getComponentVersion(ctx ocm.Context, session ocm.Session, component *actionv1.OCMComponent) (ocm.ComponentVersionAccess, error) {
	// configure the repository access
	repoSpec := genericocireg.NewRepositorySpec(ocireg.NewRepositorySpec(component.Spec.Repository.URL), nil)
	repo, err := session.LookupRepository(ctx, repoSpec)
	if err != nil {
		return nil, fmt.Errorf("repo error: %w", err)
	}

	// get the component version
	cv, err := session.LookupComponentVersion(repo, component.Spec.Name, component.Spec.Version)
	if err != nil {
		return nil, fmt.Errorf("component error: %w", err)
	}

	return cv, nil
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

	taggedURL := fmt.Sprintf("%s/%s/%s:%d", ociRegistryEndpoint, repo, resourceName, time.Now().Unix())
	log.V(4).Info("pushing joined url", "url", taggedURL)
	pusher := registry.NewClient(taggedURL)

	if err := pusher.Push(ctx, artifactPath, sourceDir, metadata); err != nil {
		return "", fmt.Errorf("failed to push artifact: %w", err)
	}
	log.V(4).Info("successfully uploaded artifact to location", "location", artifactPath, "sourcedir", sourceDir)

	return taggedURL, nil
}

func (r *OCMResourceReconciler) configureTemplateFilesystem(ctx context.Context, cv ocm.ComponentVersionAccess, resourceName string) (vfs.FileSystem, error) {
	// get the template
	_, templateBytes, err := r.getResourceForComponentVersion(cv, resourceName)
	if err != nil {
		return nil, fmt.Errorf("template error: %w", err)
	}

	// setup virtual filesystem
	virtualFS, err := osfs.NewTempFileSystem()
	if err != nil {
		return nil, fmt.Errorf("fs error: %w", err)
	}

	// extract the template
	if err := utils.ExtractTarToFs(virtualFS, templateBytes); err != nil {
		return nil, fmt.Errorf("extract tar error: %w", err)
	}

	return virtualFS, nil
}

func (r *OCMResourceReconciler) getResourceForComponentVersion(cv ocm.ComponentVersionAccess, resourceName string) (ocm.ResourceAccess, *bytes.Buffer, error) {
	resource, err := cv.GetResource(ocmmeta.NewIdentity(resourceName))
	if err != nil {
		return nil, nil, err
	}

	rd, err := cpi.ResourceReader(resource)
	if err != nil {
		return nil, nil, err
	}
	defer rd.Close()

	decompress, _, err := compression.AutoDecompress(rd)
	if err != nil {
		return nil, nil, err
	}

	data := new(bytes.Buffer)
	if _, err := data.ReadFrom(decompress); err != nil {
		return nil, nil, err
	}

	return resource, data, nil
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

func getConsumerIdentityForRepository(repo actionv1.Repository) (credentials.ConsumerIdentity, error) {
	regURL, err := url.Parse(repo.URL)
	if err != nil {
		return nil, err
	}

	if regURL.Scheme == "" {
		regURL, err = url.Parse(fmt.Sprintf("oci://%s", repo.URL))
		if err != nil {
			return nil, err
		}
	}

	return credentials.ConsumerIdentity{
		"type":     "OCIRegistry",
		"hostname": regURL.Host,
	}, nil
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
