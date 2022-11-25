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
	v1alpha1 "github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/accessmethods/localblob"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/accessmethods/ociblob"
	ocmapi "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/ocm.software/v3alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ocmruntime "github.com/open-component-model/ocm/pkg/runtime"
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
	snapshotName := fmt.Sprintf("%s/snapshots/%s:%s", r.OCIRegistryAddr, obj.Spec.SnapshotTemplate.Name, obj.Spec.SnapshotTemplate.Tag)
	digest, err := r.copyResourceToSnapshot(ctx, snapshotName, resource)
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

func (r *ResourceReconciler) copyResourceToSnapshot(ctx context.Context, snapshotName string, res *ocmapi.Resource) (string, error) {
	accessSpec := localblob.AccessSpec{}
	rawAccessSpec, err := res.Access.GetRaw()
	if err != nil {
		return "", fmt.Errorf("failed to GetRaw: %w", err)
	}

	if err := ocmruntime.DefaultJSONEncoding.Unmarshal(rawAccessSpec, &accessSpec); err != nil {
		return "", fmt.Errorf("failed to unmarshal acces spec: %w", err)
	}

	globalAccess, err := accessSpec.GlobalAccess.Evaluate(ocm.DefaultContext())
	if err != nil {
		return "", fmt.Errorf("failed to evaluate global access spec: %w", err)
	}

	ref := globalAccess.(*ociblob.AccessSpec).Reference
	sha := globalAccess.(*ociblob.AccessSpec).Digest.String()
	digest, err := name.NewDigest(fmt.Sprintf("%s:%s@%s", ref, res.GetVersion(), sha), name.Insecure)
	if err != nil {
		return "", fmt.Errorf("failed to create digest: %w", err)
	}

	// proxy image requests via the in-cluster oci-registry
	proxyURL, err := url.Parse(fmt.Sprintf("http://localhost:%d", 5001))
	if err != nil {
		return "", fmt.Errorf("failed to parse oci registry url: %w", err)
	}

	// create a transport to the in-cluster oci-registry
	tr := newCustomTransport(remote.DefaultTransport.(*http.Transport).Clone(), proxyURL)

	// set context values to be transmitted as headers on the registry requests
	for k, v := range map[string]string{
		"registry":   digest.Repository.Registry.String(),
		"repository": digest.Repository.String(),
		"digest":     digest.String(),
		"image":      digest.Name(),
		"tag":        res.Version,
	} {
		ctx = context.WithValue(ctx, contextKey(k), v)
	}

	// fetch the layer
	layer, err := remote.Layer(digest, remote.WithTransport(tr), remote.WithContext(ctx))
	if err != nil {
		return "", fmt.Errorf("failed to get component object: %w", err)
	}

	// create snapshot with single layer
	snapshot, err := mutate.AppendLayers(empty.Image, layer)
	if err != nil {
		return "", fmt.Errorf("failed to get append layer: %w", err)
	}

	snapshotRef, err := name.ParseReference(snapshotName, name.Insecure)
	if err != nil {
		return "", fmt.Errorf("failed to create snapshot reference: %w", err)
	}

	ct := time.Now()
	snapshotMeta := ociclient.Metadata{
		Created: ct.Format(time.RFC3339),
	}

	// add metadata
	snapshot = mutate.Annotations(snapshot, snapshotMeta.ToAnnotations()).(gcrv1.Image)

	// write snapshot to registry
	if err := remote.Write(snapshotRef, snapshot); err != nil {
		return "", fmt.Errorf("failed to write snapshot: %w", err)
	}

	return digest.DigestStr(), nil
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
