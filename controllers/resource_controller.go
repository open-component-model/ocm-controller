// Copyright 2022.
// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	ociclient "github.com/fluxcd/pkg/oci/client"
	"github.com/google/go-containerregistry/pkg/name"
	gcrv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/stream"
	v1alpha1 "github.com/open-component-model/ocm-controller/api/v1alpha1"
	ocmclient "github.com/open-component-model/ocm-controller/pkg/ocm"
	ocmmetav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	ocmapi "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/ocm.software/v3alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type contextKey string

// ResourceReconciler reconciles a Resource object
type ResourceReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	OCIRegistryAddr string
	OCMClient       ocmclient.FetchVerifier
}

//+kubebuilder:rbac:groups=delivery.ocm.software,resources=resources,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=delivery.ocm.software,resources=resources/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=delivery.ocm.software,resources=resources/finalizers,verbs=update

// SetupWithManager sets up the controller with the Manager.
func (r *ResourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Resource{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(r)
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ResourceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithName("resource-controller")

	log.Info("starting resource reconcile loop")
	resource := &v1alpha1.Resource{}
	if err := r.Client.Get(ctx, req.NamespacedName, resource); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get resource object: %w", err)
	}
	log.Info("found resource", "resource", resource)

	return r.reconcile(ctx, resource)
}

func (r *ResourceReconciler) reconcile(ctx context.Context, obj *v1alpha1.Resource) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithName("resource-controller")

	log.Info("finding component ref", "resource", obj)

	// read component version
	cdvKey := types.NamespacedName{
		Name:      obj.Spec.ComponentVersionRef.Name,
		Namespace: obj.Spec.ComponentVersionRef.Namespace,
	}

	componentVersion := &v1alpha1.ComponentVersion{}
	if err := r.Get(ctx, cdvKey, componentVersion); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, nil
		}

		return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()},
			fmt.Errorf("failed to get component version: %w", err)
	}

	log.Info("got component version", "component version", cdvKey.String())

	// Initialize the patch helper.
	patchHelper, err := patch.NewHelper(obj, r.Client)
	if err != nil {
		return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, err
	}

	componentDescriptor, err := GetComponentDescriptor(ctx, r.Client, obj.Spec.Resource.ReferencePath, componentVersion.Status.ComponentDescriptor)
	if err != nil {
		return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()},
			fmt.Errorf("failed to get component descriptor: %w", err)
	}

	resource := componentDescriptor.GetResource(obj.Spec.Resource.Name)
	if resource == nil {
		return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, nil
	}

	// push the resource snapshot to oci
	//snapshotName := fmt.Sprintf("%s/snapshots/%s:%s", r.OCIRegistryAddr, obj.Spec.SnapshotTemplate.Name, obj.Spec.SnapshotTemplate.Tag)
	snapshotName := fmt.Sprintf("%s/snapshots/%s/%s/%s/%s", r.OCIRegistryAddr, componentVersion.Spec.Component, componentVersion.Spec.Version, resource.Name, resource.Version)
	log.Info("pushing resource to snapshot", "snapshot-name", snapshotName)
	digest, err := r.copyResourceToSnapshot(ctx, componentVersion, snapshotName, resource)
	if err != nil {
		return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, err
	}

	//TODO@souleb: Create the cr before attempting to push to registry and set a condition accordingly
	// This also means that we need to check for the existence of the cr before attempting to push to the registry
	// and if a cr exists, we need to check if the digest matches the one in the cr

	// create/update the snapshot custom resource
	snapshotCR := &v1alpha1.Snapshot{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: obj.GetNamespace(),
			Name:      obj.Spec.SnapshotTemplate.Name,
		},
	}

	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, snapshotCR, func() error {
		if snapshotCR.ObjectMeta.CreationTimestamp.IsZero() {
			if err := controllerutil.SetOwnerReference(obj, snapshotCR, r.Scheme); err != nil {
				return fmt.Errorf("failed to set owner to snapshot object: %w", err)
			}
		}
		snapshotCR.Spec = v1alpha1.SnapshotSpec{
			Ref:    strings.TrimPrefix(snapshotName, r.OCIRegistryAddr+"/snapshots/"),
			Digest: digest,
		}
		return nil
	})

	if err != nil {
		return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()},
			fmt.Errorf("failed to create or update component descriptor: %w", err)
	}

	obj.Status.LastAppliedResourceVersion = resource.Version

	log.Info("sucessfully created snapshot", "name", snapshotName)

	obj.Status.ObservedGeneration = obj.GetGeneration()

	if err := patchHelper.Patch(ctx, obj); err != nil {
		return ctrl.Result{
			RequeueAfter: obj.GetRequeueAfter(),
		}, fmt.Errorf("failed to patch resource and set snaphost value: %w", err)
	}

	log.Info("successfully reconciled resource", "name", obj.GetName())

	return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, nil
}

func (r *ResourceReconciler) copyResourceToSnapshot(ctx context.Context, componentVersion *v1alpha1.ComponentVersion, snapshotName string, res *ocmapi.Resource) (string, error) {
	//access, err := GetImageReference(res)
	//if err != nil {
	//	return "", fmt.Errorf("failed to create digest: %w", err)
	//}
	cv, err := r.OCMClient.GetComponentVersion(ctx, componentVersion, componentVersion.Spec.Component, componentVersion.Spec.Version)
	if err != nil {
		return "", fmt.Errorf("failed to get component version: %w", err)
	}

	resource, err := cv.GetResource(ocmmetav1.NewIdentity(res.Name))
	if err != nil {
		return "", fmt.Errorf("failed to fetch resource: %w", err)
	}

	access, err := resource.AccessMethod()
	if err != nil {
		return "", fmt.Errorf("failed to fetch access spec: %w", err)
	}
	reader, err := access.Reader()
	if err != nil {
		return "", fmt.Errorf("failed to fetch reader: %w", err)
	}

	// reference:version@sha
	// The problem here is that we don't just want to deal with OCI objects. So the repository is something we construct
	// out of an access method. However, this NewRepository thing requires an actual repository. That is fine. Maybe it
	// can be a local space?

	// proxy image requests via the in-cluster oci-registry
	proxyURL, err := url.Parse(fmt.Sprintf("http://localhost:%d", 5000))
	if err != nil {
		return "", fmt.Errorf("failed to parse oci registry url: %w", err)
	}

	// TODO: Change localhost:5000 to the registry service name.
	//repoName := fmt.Sprintf("localhost:5000/%s/%s/%s/%s", componentVersion.Spec.Component, componentVersion.Spec.Version, resource.Meta().Name, resource.Meta().Version)
	//localhost:5000/github.com/phoban01/webpage/webpage/v1.0.0@digest
	repo, err := name.NewRepository(snapshotName, name.Insecure)
	if err != nil {
		return "", fmt.Errorf("failed to create digest: %w", err)
	}

	layer := stream.NewLayer(reader)
	// create a transport to the in-cluster oci-registry
	tr := newCustomTransport(remote.DefaultTransport.(*http.Transport).Clone(), proxyURL)

	// set context values to be transmitted as headers on the registry requests
	for k, v := range map[string]string{
		"registry":   "localhost:5000",
		"repository": snapshotName,
		"tag":        resource.Meta().Version,
	} {
		ctx = context.WithValue(ctx, contextKey(k), v)
	}

	// create snapshot with single layer
	snapshot, err := mutate.AppendLayers(empty.Image, layer)
	if err != nil {
		return "", fmt.Errorf("failed to get append layer: %w", err)
	}

	ct := time.Now()
	snapshotMeta := ociclient.Metadata{
		Created: ct.Format(time.RFC3339),
	}

	// add metadata
	snapshot = mutate.Annotations(snapshot, snapshotMeta.ToAnnotations()).(gcrv1.Image)
	layers, err := snapshot.Layers()
	if err != nil {
		return "", fmt.Errorf("failed to get first layer: %w", err)
	}

	if err := remote.WriteLayer(repo, layers[0], remote.WithTransport(tr), remote.WithContext(ctx)); err != nil {
		return "", fmt.Errorf("failed to write layer: %w", err)
	}

	digest, err := layers[0].Digest()
	if err != nil {
		return "", fmt.Errorf("failed to get digest from layer: %w", err)
	}
	return digest.String(), nil
}

type customTransport struct {
	http.RoundTripper
}

func newCustomTransport(upstream *http.Transport, proxyURL *url.URL) *customTransport {
	upstream.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	upstream.Proxy = http.ProxyURL(proxyURL)
	upstream.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}
	return &customTransport{upstream}
}

func (ct *customTransport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	keys := []string{"digest", "registry", "repository", "tag", "image"}
	for _, key := range keys {
		value := req.Context().Value(contextKey(key))
		if value != nil {
			req.Header.Set("x-"+key, value.(string))
		}
	}
	return ct.RoundTripper.RoundTrip(req)
}
