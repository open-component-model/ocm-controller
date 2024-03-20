// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/patch"
	"github.com/mandelsoft/vfs/pkg/osfs"
	"github.com/mandelsoft/vfs/pkg/projectionfs"
	"github.com/open-component-model/ocm-controller/pkg/metrics"
	"golang.org/x/exp/slog"
	"gopkg.in/yaml.v2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kuberecorder "k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/pkg/ocm"
	"github.com/open-component-model/ocm-controller/pkg/snapshot"
	wasmruntime "github.com/open-component-model/ocm-controller/pkg/wasm/runtime"
	"github.com/open-component-model/ocm/pkg/common"
	ocmcore "github.com/open-component-model/ocm/pkg/contexts/ocm"
	ocmmetav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/download/handlers/dirtree"
	ocmreg "github.com/open-component-model/ocm/pkg/contexts/ocm/repositories/ocireg"
)

// ResourcePipelineReconciler reconciles a ResourcePipeline object.
type ResourcePipelineReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	kuberecorder.EventRecorder
	OCMClient      ocm.Contract
	SnapshotWriter snapshot.Writer
}

// SetupWithManager sets up the controller with the Manager.
func (r *ResourcePipelineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ResourcePipeline{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(r)
}

//+kubebuilder:rbac:groups=delivery.ocm.software,resources=resourcepipelines,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=external-secrets.io,resources=secretstores;secretstores/status,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=delivery.ocm.software,resources=resourcepipelines/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=delivery.ocm.software,resources=resourcepipelines/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ResourcePipelineReconciler) Reconcile(
	ctx context.Context,
	req ctrl.Request,
) (retResult ctrl.Result, retErr error) {
	logger := log.FromContext(ctx).WithName("resource-pipeline-controller")

	obj := &v1alpha1.ResourcePipeline{}
	if err := r.Client.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to get resource pipeline: %w", err)
	}

	if obj.Spec.Suspend {
		logger.Info("resource object suspended")

		return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, nil
	}

	patchHelper := patch.NewSerialPatcher(obj, r.Client)

	// Always attempt to patch the object and status after each reconciliation.
	defer func() {
		// Patching has not been set up, or the controller errored earlier.
		if patchHelper == nil {
			return
		}

		if condition := conditions.Get(obj, meta.StalledCondition); condition != nil &&
			condition.Status == metav1.ConditionTrue {
			conditions.Delete(obj, meta.ReconcilingCondition)
		}

		// Check if it's a successful reconciliation.
		// We don't set Requeue in case of error, so we can safely check for Requeue.
		if retErr == nil {
			// Remove the reconciling condition if it's set.
			conditions.Delete(obj, meta.ReconcilingCondition)

			// Set the return err as the ready failure message if the resource is not ready, but also not reconciling or stalled.
			if ready := conditions.Get(obj, meta.ReadyCondition); ready != nil &&
				ready.Status == metav1.ConditionFalse &&
				!conditions.IsStalled(obj) {
				retErr = errors.New(conditions.GetMessage(obj, meta.ReadyCondition))
			}
		}

		// If still reconciling then reconciliation did not succeed, set to ProgressingWithRetry to
		// indicate that reconciliation will be retried.
		if conditions.IsReconciling(obj) {
			reconciling := conditions.Get(obj, meta.ReconcilingCondition)
			reconciling.Reason = meta.ProgressingWithRetryReason
			conditions.Set(obj, reconciling)
		}

		// If not reconciling or stalled than mark Ready=True
		if !conditions.IsReconciling(obj) && !conditions.IsStalled(obj) && retErr == nil {
			conditions.MarkTrue(
				obj,
				meta.ReadyCondition,
				meta.SucceededReason,
				"Reconciliation success",
			)
		}
		// Set status observed generation option if the component is stalled or ready.
		if conditions.IsStalled(obj) || conditions.IsReady(obj) {
			obj.Status.ObservedGeneration = obj.Generation
		}

		// Update the object.
		if perr := patchHelper.Patch(ctx, obj); perr != nil {
			retErr = errors.Join(retErr, perr)
		}
	}()

	// if the snapshot name has not been generated then
	// generate, patch the status and requeue
	if obj.GetSnapshotName() == "" {
		name, err := snapshot.GenerateSnapshotName(obj.GetName())
		if err != nil {
			return ctrl.Result{}, err
		}
		obj.Status.SnapshotName = name

		return ctrl.Result{Requeue: true}, nil
	}

	// Remove any stale Ready condition, most likely False, set above. Its value
	// is derived from the overall result of the reconciliation in the deferred
	// block at the very end.
	conditions.Delete(obj, meta.ReadyCondition)

	return r.reconcile(ctx, obj)
}

func (r *ResourcePipelineReconciler) reconcile(
	ctx context.Context,
	obj *v1alpha1.ResourcePipeline,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithName("resource-pipeline-controller")
	conditions.MarkTrue(
		obj,
		meta.ReconcilingCondition,
		meta.ProgressingReason,
		"Reconciliation in progress",
	)

	// get the component
	cv, err := r.getComponentVersionAccess(ctx, obj)
	if err != nil {
		if apierrors.IsNotFound(err) || errors.Is(err, errCVNotReady) {
			logger.Info("cv not found or not ready: retrying", "msg", err.Error())

			return ctrl.Result{
				RequeueAfter: obj.GetRequeueAfter(),
			}, nil
		}

		return ctrl.Result{}, fmt.Errorf("could not get component version: %w", err)
	}
	defer cv.Close()

	// get the resource
	res, err := cv.GetResource(ocmmetav1.NewIdentity(obj.Spec.SourceRef.ResourceRef.Name))
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("resource not found: %w", err)
	}

	// prepare VFS
	dir, err := os.MkdirTemp("", "wasm-tmp-")
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get temp folder: %w", err)
	}
	defer os.RemoveAll(dir)

	tmpfs, err := projectionfs.New(osfs.New(), dir)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create temp folder: %w", err)
	}

	// download resource to VFS
	_, _, err = dirtree.New().Download(common.NewPrinter(os.Stdout), res, "", tmpfs)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to download resource: %w", err)
	}

	parameters := map[string]any{}
	if obj.Spec.Parameters != nil {
		if err := json.Unmarshal(obj.Spec.Parameters.Raw, &parameters); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to parse parameters: %w", err)
		}
	}

	wasmRun := wasmruntime.New().
		WithLogger(slog.New(slog.NewJSONHandler(os.Stdout, nil))).
		WithComponent(cv).
		WithDir(dir)
	if err := wasmRun.Init(ctx); err != nil {
		return ctrl.Result{}, fmt.Errorf("could not create wasm runtime: %w", err)
	}
	defer wasmRun.Close(ctx)

	for _, step := range obj.Spec.PipelineSpec.Steps {
		if err := r.executePipelineStepWithTimeout(ctx, wasmRun, step, parameters); err != nil {
			return ctrl.Result{}, fmt.Errorf(
				"failed to execute pipeline step %s: %w",
				step.Name,
				err,
			)
		}
		logger.Info(fmt.Sprintf("step completed: %s", step.Name))
	}

	id, err := r.getIdentity(ctx, obj.Spec.SourceRef)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get identity for source ref: %w", err)
	}

	digest, size, err := r.SnapshotWriter.Write(ctx, obj, dir, id)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create write snapshot: %w", err)
	}

	obj.Status.LatestSnapshotDigest = digest
	metrics.SnapshotNumberOfBytesReconciled.WithLabelValues(obj.GetSnapshotName(), digest, cv.GetName()).Set(float64(size))

	conditions.Delete(obj, meta.ReconcilingCondition)

	return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, nil
}

var errCVNotReady = errors.New("component version not ready")

func (r *ResourcePipelineReconciler) getComponentVersionAccess(
	ctx context.Context,
	obj *v1alpha1.ResourcePipeline,
) (ocmcore.ComponentVersionAccess, error) {
	var componentVersion v1alpha1.ComponentVersion
	key := types.NamespacedName{
		Name:      obj.Spec.SourceRef.Name,
		Namespace: obj.Spec.SourceRef.Namespace,
	}
	if err := r.Get(ctx, key, &componentVersion); err != nil {
		return nil, err
	}

	if !conditions.IsReady(&componentVersion) {
		return nil, errCVNotReady
	}

	octx, err := r.OCMClient.CreateAuthenticatedOCMContext(ctx, &componentVersion)
	if err != nil {
		return nil, fmt.Errorf("could not create auth context: %w", err)
	}

	return r.OCMClient.GetComponentVersion(
		ctx,
		octx,
		&componentVersion,
		componentVersion.GetComponentName(),
		componentVersion.GetVersion(),
	)
}

func (r *ResourcePipelineReconciler) executePipelineStepWithTimeout(ctx context.Context,
	wr *wasmruntime.Runtime,
	step v1alpha1.WasmStep,
	parameters map[string]any,
) error {
	ctx, cancel := context.WithTimeout(ctx, step.Timeout.Duration)
	defer cancel()

	return r.executePipelineStep(ctx, wr, step, parameters)
}

// for each WASM step
// - get the module
// - TODO: verify the signature
// - execute.
func (r *ResourcePipelineReconciler) executePipelineStep(
	ctx context.Context,
	wr *wasmruntime.Runtime,
	step v1alpha1.WasmStep,
	parameters map[string]any,
) error {
	data, err := r.fetchWasmBlob(step)
	if err != nil {
		return fmt.Errorf("failed to fetch wasm script for %s: %w", step.Name, err)
	}

	values := map[string]any{}
	if step.Values != nil {
		if err := json.Unmarshal(step.Values.Raw, &values); err != nil {
			return fmt.Errorf("failed to parse raw values: %w", err)
		}
	}
	if err := processValueFunctions(parameters, values); err != nil {
		return fmt.Errorf("failed to process value functions: %w", err)
	}

	mergedValues, err := yaml.Marshal(values)
	if err != nil {
		return fmt.Errorf("failed interpret values functions for step %s: %w", step.Name, err)
	}

	if err := wr.Call(ctx, step.Name, data, string(mergedValues)); err != nil {
		return fmt.Errorf("failed to run step %s: %w", step.Name, err)
	}

	return nil
}

func (r *ResourcePipelineReconciler) fetchWasmBlob(
	step v1alpha1.WasmStep,
) (_ []byte, err error) {
	octx := ocmcore.New()

	repo, err := octx.RepositoryForSpec(ocmreg.NewRepositorySpec(step.Registry, nil))
	if err != nil {
		return nil, fmt.Errorf("failed to get repository for step %s: %w", step.Name, err)
	}
	defer func() {
		if cerr := repo.Close(); cerr != nil {
			err = errors.Join(err, cerr)
		}
	}()

	ocmWASMCV, err := repo.LookupComponentVersion(step.GetComponent(), step.GetComponentVersion())
	if err != nil {
		return nil, fmt.Errorf("failed to lookup verions for step %s: %w", step.Name, err)
	}
	defer func() {
		if cerr := ocmWASMCV.Close(); cerr != nil {
			err = errors.Join(err, cerr)
		}
	}()

	wasmRes, err := ocmWASMCV.GetResource(ocmmetav1.NewIdentity(step.GetResource()))
	if err != nil {
		return nil, fmt.Errorf("failed to get resource for step %s: %w", step.Name, err)
	}

	meth, err := wasmRes.AccessMethod()
	if err != nil {
		return nil, fmt.Errorf("failed to run access method for step %s: %w", step.Name, err)
	}

	data, err := meth.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to get resource data for step %s: %w", step.Name, err)
	}

	if err := meth.Close(); err != nil {
		return nil, fmt.Errorf("failed to close access method for step %s: %w", step.Name, err)
	}

	return data, nil
}

const (
	parametersInjectKey = "$parameters"
)

func parametersInjectFunc(parameters map[string]any, key string) (any, error) {
	if v, ok := parameters[key]; ok {
		return v, nil
	}

	return nil, fmt.Errorf("parameter with key %s not found", key)
}

func injectValue(parameters map[string]any, value string) (any, error) {
	idx := strings.Index(value, ".")
	if idx < 0 {
		return nil, fmt.Errorf(
			"expected inject function name to be of format $func.key but was %s",
			value,
		)
	}

	if idx+1 >= len(value) {
		return nil, fmt.Errorf("missing value from func key: %s", value)
	}

	funcName, key := value[:idx], value[idx+1:]

	if strings.HasPrefix(funcName, parametersInjectKey) {
		return parametersInjectFunc(parameters, key)
	}

	return nil, fmt.Errorf("unknown inject function: %s", funcName)
}

// processValueFunctions takes a secret map and parameters map then starts updating the values
// based on placeholders in the values like `$secrets.kubeconfig` or `$parameters.replicas`.
// Note that this function assumes that neither the parameters nor the secrets map contains
// nested values. This is true for secrets as secrets contains a Key, but parameters is
// technically an arbitrary structure.
func processValueFunctions(
	parameters map[string]any,
	values map[string]any,
) error {
	for k, v := range values {
		switch t := v.(type) {
		case string:
			if err := injectMapValue(t, values, k, parameters); err != nil {
				return fmt.Errorf("failed to inject value: %w", err)
			}
		case map[string]any:
			if err := processValueFunctions(parameters, t); err != nil {
				return fmt.Errorf("failed to process values: %w", err)
			}
		case []any:
			for i, e := range t {
				switch et := e.(type) {
				case string:
					if err := injectSliceValue(et, t, i, parameters); err != nil {
						return fmt.Errorf("failed to inject value: %w", err)
					}
				case map[string]any:
					if err := processValueFunctions(parameters, et); err != nil {
						return fmt.Errorf("failed to process values: %w", err)
					}
				}
			}
		}
	}

	return nil
}

func injectMapValue(t string, values map[string]any, k string, parameters map[string]any) error {
	if !strings.HasPrefix(t, "$") {
		return nil
	}

	inject, err := injectValue(parameters, t)
	if err != nil {
		return err
	}

	values[k] = inject

	return nil
}

func injectSliceValue(t string, values []any, i int, parameters map[string]any) error {
	if !strings.HasPrefix(t, "$") {
		return nil
	}

	inject, err := injectValue(parameters, t)
	if err != nil {
		return fmt.Errorf("failed to inject value: %w", err)
	}

	values[i] = inject

	return nil
}

func (r *ResourcePipelineReconciler) getIdentity(
	ctx context.Context,
	obj v1alpha1.ObjectReference,
) (ocmmetav1.Identity, error) {
	var (
		id  ocmmetav1.Identity
		err error
	)

	key := types.NamespacedName{
		Name:      obj.Name,
		Namespace: obj.Namespace,
	}

	cv := &v1alpha1.ComponentVersion{}
	if err := r.Client.Get(ctx, key, cv); err != nil {
		return nil, err
	}

	id = ocmmetav1.Identity{
		v1alpha1.ComponentNameKey:    cv.Status.ComponentDescriptor.ComponentDescriptorRef.Name,
		v1alpha1.ComponentVersionKey: cv.Status.ComponentDescriptor.Version,
		v1alpha1.ResourceNameKey:     obj.ResourceRef.Name,
		v1alpha1.ResourceVersionKey:  obj.ResourceRef.Version,
	}

	// apply the extra identity fields if provided
	for k, v := range obj.ResourceRef.ExtraIdentity {
		id[k] = v
	}

	return id, err
}
