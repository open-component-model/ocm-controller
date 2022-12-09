// Copyright 2022.
// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"fmt"
	"strings"

	"github.com/open-component-model/ocm/pkg/contexts/ocm/utils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	ocmmetav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	ocmapi "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/ocm.software/v3alpha1"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/pkg/oci"
	ocmclient "github.com/open-component-model/ocm-controller/pkg/ocm"
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
	if componentDescriptor == nil {
		return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, fmt.Errorf("component version with name '%s' is not yet available, retrying", componentVersion.Name)
	}
	if err != nil {
		return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()},
			fmt.Errorf("failed to get component descriptor: %w", err)
	}
	resource := componentDescriptor.GetResource(obj.Spec.Resource.Name)
	if resource == nil {
		return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, nil
	}

	// push the resource snapshot to oci
	repositoryName := fmt.Sprintf(
		"%s/%s/%s",
		r.OCIRegistryAddr,
		obj.Namespace,
		obj.Spec.SnapshotTemplate.Name,
	)
	log.V(4).Info("creating snapshot with name", "snapshot-name", repositoryName)
	digest, err := r.copyResourceToSnapshot(ctx, componentVersion, repositoryName, obj.ResourceVersion, resource, obj.Spec.Resource.ReferencePath)
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
			Ref: strings.TrimPrefix(repositoryName, r.OCIRegistryAddr+"/"),
		}
		return nil
	})
	if err != nil {
		return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()},
			fmt.Errorf("failed to create or update component descriptor: %w", err)
	}

	newSnapshotCR := snapshotCR.DeepCopy()
	newSnapshotCR.Status.Digest = digest
	newSnapshotCR.Status.Tag = obj.ResourceVersion
	if err := patchObject(ctx, r.Client, snapshotCR, newSnapshotCR); err != nil {
		return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()},
			fmt.Errorf("failed to patch snapshot CR: %w", err)
	}

	obj.Status.LastAppliedResourceVersion = resource.Version

	log.Info("successfully created snapshot", "name", repositoryName)

	obj.Status.ObservedGeneration = obj.GetGeneration()

	if err := patchHelper.Patch(ctx, obj); err != nil {
		return ctrl.Result{
			RequeueAfter: obj.GetRequeueAfter(),
		}, fmt.Errorf("failed to patch resource and set snaphost value: %w", err)
	}

	log.Info("successfully reconciled resource", "name", obj.GetName())

	return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, nil
}

func (r *ResourceReconciler) copyResourceToSnapshot(ctx context.Context, componentVersion *v1alpha1.ComponentVersion, repositoryName, tag string, res *ocmapi.Resource, referencePath []map[string]string) (string, error) {
	cv, err := r.OCMClient.GetComponentVersion(ctx, componentVersion, componentVersion.Spec.Component, componentVersion.Status.ReconciledVersion)
	if err != nil {
		return "", fmt.Errorf("failed to get component version: %w", err)
	}
	defer cv.Close()

	var identities []ocmmetav1.Identity
	for _, ref := range referencePath {
		identities = append(identities, ref)
	}

	resource, _, err := utils.ResolveResourceReference(cv, ocmmetav1.NewNestedResourceRef(ocmmetav1.NewIdentity(res.Name), identities), cv.Repository())
	if err != nil {
		return "", fmt.Errorf("failed to resolve reference path to resource: %w", err)
	}

	access, err := resource.AccessMethod()
	if err != nil {
		return "", fmt.Errorf("failed to fetch access spec: %w", err)
	}

	reader, err := access.Reader()
	if err != nil {
		return "", fmt.Errorf("failed to fetch reader: %w", err)
	}

	repo, err := oci.NewRepository(repositoryName, oci.WithInsecure())
	if err != nil {
		return "", fmt.Errorf("failed create new repository: %w", err)
	}

	// TODO: add extra identity
	digest, err := repo.PushStreamingImage(tag, reader, "", nil)
	if err != nil {
		return "", fmt.Errorf("failed to push image: %w", err)
	}

	return digest, nil
}
