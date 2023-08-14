// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/mandelsoft/vfs/pkg/osfs"
	ocmcore "github.com/open-component-model/ocm/pkg/contexts/ocm"
	ocmreg "github.com/open-component-model/ocm/pkg/contexts/ocm/repositories/ocireg"
	"golang.org/x/exp/slog"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kuberecorder "k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/mandelsoft/vfs/pkg/projectionfs"
	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	deliveryv1alpha1 "github.com/open-component-model/ocm-controller/api/v1alpha1"
	wasmruntime "github.com/open-component-model/ocm-controller/internal/wasm/runtime"
	"github.com/open-component-model/ocm-controller/pkg/ocm"
	"github.com/open-component-model/ocm/pkg/common"
	ocmmetav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/download/handlers/dirtree"
)

// ResourcePipelineReconciler reconciles a ResourcePipeline object
type ResourcePipelineReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	kuberecorder.EventRecorder
	OCMClient ocm.Contract
}

// SetupWithManager sets up the controller with the Manager.
func (r *ResourcePipelineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&deliveryv1alpha1.ResourcePipeline{}).
		Complete(r)
}

//+kubebuilder:rbac:groups=delivery.ocm.software,resources=resourcepipelines,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=delivery.ocm.software,resources=resourcepipelines/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=delivery.ocm.software,resources=resourcepipelines/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ResourcePipelineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// logger := log.FromContext(ctx).WithName("resource-pipeline-controller")

	obj := &v1alpha1.ResourcePipeline{}
	if err := r.Client.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to get resource pipeline: %w", err)
	}

	// if obj.Spec.Suspend {
	//     logger.Info("resource object suspended")
	//     return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, nil
	// }

	return r.reconcile(ctx, obj)
}

func (r *ResourcePipelineReconciler) reconcile(ctx context.Context, obj *v1alpha1.ResourcePipeline) (ctrl.Result, error) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// get the component
	cv, err := r.getComponentVersionAccess(ctx, obj)
	if err != nil {
		return ctrl.Result{}, err
	}
	defer cv.Close()

	// get the resource
	res, err := cv.GetResource(ocmmetav1.NewIdentity(obj.Spec.SourceRef.Resource))
	if err != nil {
		return ctrl.Result{}, err
	}

	// prepare VFS
	dir, err := os.MkdirTemp("", "wasm-tmp-")
	if err != nil {
		return ctrl.Result{}, err
	}
	defer os.RemoveAll(dir)

	tmpfs, err := projectionfs.New(osfs.New(), dir)
	if err != nil {
		os.Remove(dir)
		return ctrl.Result{}, err
	}

	// download resource to VFS
	_, _, err = dirtree.New().Download(common.NewPrinter(os.Stdout), res, "", tmpfs)
	if err != nil {
		return ctrl.Result{}, err
	}

	// for each WASM step
	// - get the module
	// - verify the signature
	// - execute
	for _, step := range obj.Spec.PipelineSpec.Steps {
		octx := ocmcore.New()
		repo, err := octx.RepositoryForSpec(ocmreg.NewRepositorySpec(step.Registry, nil))
		if err != nil {
			return ctrl.Result{}, err
		}
		defer repo.Close()

		ocmWASMCV, err := repo.LookupComponentVersion(step.GetComponent(), step.GetComponentVersion())
		if err != nil {
			return ctrl.Result{}, err
		}
		defer ocmWASMCV.Close()

		wasmRes, err := ocmWASMCV.GetResource(ocmmetav1.NewIdentity(step.GetResource()))
		if err != nil {
			return ctrl.Result{}, err
		}

		meth, err := wasmRes.AccessMethod()
		if err != nil {
			return ctrl.Result{}, err
		}

		data, err := meth.Get()
		if err != nil {
			return ctrl.Result{}, err
		}

		mod := wasmruntime.NewModule(step.Name, logger, cv, dir)
		defer mod.Close()

		if err := mod.Run(ctx, step.Values.Raw, data); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, nil
}

func (r ResourcePipelineReconciler) getComponentVersionAccess(ctx context.Context, obj *v1alpha1.ResourcePipeline) (ocmcore.ComponentVersionAccess, error) {
	var componentVersion v1alpha1.ComponentVersion
	key := types.NamespacedName{
		Name:      obj.Spec.SourceRef.Name,
		Namespace: obj.Spec.SourceRef.Namespace,
	}
	if err := r.Get(ctx, key, &componentVersion); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, err
		}

		err = fmt.Errorf("failed to get component version: %w", err)
		return nil, err
	}

	if !conditions.IsReady(&componentVersion) {
		return nil, errors.New("component is not ready")
	}

	octx, err := r.OCMClient.CreateAuthenticatedOCMContext(ctx, &componentVersion)
	if err != nil {
		return nil, err
	}

	return r.OCMClient.GetComponentVersion(ctx, octx, &componentVersion, componentVersion.GetComponentName(), componentVersion.GetVersion())
}
