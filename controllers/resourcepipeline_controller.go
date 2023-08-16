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

	esapi "github.com/external-secrets/external-secrets/apis/externalsecrets/v1beta1"
	esv1beta1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1beta1"
	"github.com/external-secrets/external-secrets/pkg/controllers/secretstore"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/patch"
	"github.com/mandelsoft/vfs/pkg/osfs"
	"github.com/mandelsoft/vfs/pkg/projectionfs"
	"golang.org/x/exp/slog"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kuberecorder "k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	deliveryv1alpha1 "github.com/open-component-model/ocm-controller/api/v1alpha1"
	wasmruntime "github.com/open-component-model/ocm-controller/internal/wasm/runtime"
	"github.com/open-component-model/ocm-controller/pkg/ocm"
	"github.com/open-component-model/ocm/pkg/common"
	ocmcore "github.com/open-component-model/ocm/pkg/contexts/ocm"
	ocmmetav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/download/handlers/dirtree"
	ocmreg "github.com/open-component-model/ocm/pkg/contexts/ocm/repositories/ocireg"
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
//+kubebuilder:rbac:groups=external-secrets.io,resources=secretstores;secretstores/status,verbs=get;list;watch;create;update;patch;delete
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
	wasmLogger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// get the component
	cv, err := r.getComponentVersionAccess(ctx, obj)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("component version not found: %w", err)
	}
	defer cv.Close()

	// get the resource
	res, err := cv.GetResource(ocmmetav1.NewIdentity(obj.Spec.SourceRef.Resource))
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
		os.Remove(dir)
		return ctrl.Result{}, fmt.Errorf("failed to create temp folder: %w", err)
	}

	// download resource to VFS
	_, _, err = dirtree.New().Download(common.NewPrinter(os.Stdout), res, "", tmpfs)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to download resource: %w", err)
	}

	// Build the secret store manager and start building up a map of secret values.
	secretManager := secretstore.NewManager(r.Client, "default", true)
	defer secretManager.Close(ctx)

	secretMap := make(map[string]any)

	for k, secret := range obj.Spec.Secrets {
		store := &esv1beta1.SecretStore{}
		if err := r.Get(ctx, types.NamespacedName{
			Name:      secret.SecretStoreRef.Name,
			Namespace: secret.SecretStoreRef.Namespace,
		}, store); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to find secret store: %w", err)
		}

		storeProvider, err := esapi.GetProvider(store)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to get store provider: %w", err)
		}

		secretPatchers := patch.NewSerialPatcher(store, r.Client)

		// taken from ESO's validation status messages.
		msgStoreValidated := "store validated"
		cond := secretstore.NewSecretStoreCondition(esapi.SecretStoreReady, v1.ConditionTrue, esapi.ReasonStoreValid, msgStoreValidated)
		status := store.GetStatus()
		status.Conditions = append(status.Conditions, *cond)
		status.Capabilities = storeProvider.Capabilities()
		store.SetStatus(status)
		if err := secretPatchers.Patch(ctx, store); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to patch store: %w", err)
		}

		secrets, err := secretManager.Get(ctx, esv1beta1.SecretStoreRef{
			Name: secret.SecretStoreRef.Name,
			Kind: store.Kind,
		}, secret.SecretStoreRef.Namespace, nil)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to retrieve secret: %w", err)
		}

		value, err := secrets.GetSecret(ctx, secret.RemoteRef)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to get secret: %w", err)
		}

		// build up a map of key/property values.
		secretMap[k] = string(value)
	}

	parameters := map[string]any{}
	if err := json.Unmarshal(obj.Spec.Parameters.Raw, &parameters); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to parse parameters: %w", err)
	}

	// for each WASM step
	// - get the module
	// - verify the signature
	// - execute
	for _, step := range obj.Spec.PipelineSpec.Steps {
		octx := ocmcore.New()
		repo, err := octx.RepositoryForSpec(ocmreg.NewRepositorySpec(step.Registry, nil))
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to get repositroy for step %s: %w", step.Name, err)
		}
		defer repo.Close()

		ocmWASMCV, err := repo.LookupComponentVersion(step.GetComponent(), step.GetComponentVersion())
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to lookup verions for step %s: %w", step.Name, err)
		}
		defer ocmWASMCV.Close()

		wasmRes, err := ocmWASMCV.GetResource(ocmmetav1.NewIdentity(step.GetResource()))
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to get resource for step %s: %w", step.Name, err)
		}

		meth, err := wasmRes.AccessMethod()
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to run access method for step %s: %w", step.Name, err)
		}

		data, err := meth.Get()
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to get resource data for step %s: %w", step.Name, err)
		}

		mod := wasmruntime.NewModule(step.Name, wasmLogger, obj, cv, dir)
		defer mod.Close()

		values := map[string]any{}
		if err := json.Unmarshal(step.Values.Raw, &values); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to parse raw values: %w", err)
		}

		if err := processValueFunctions(secretMap, parameters, values); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to process value functions: %w", err)
		}

		mergedValues, err := json.Marshal(values)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed interpret values functions for step %s: %w", step.Name, err)
		}

		if err := mod.Run(ctx, mergedValues, data); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to run step %s: %w", step.Name, err)
		}
	}

	return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, nil
}

func (r *ResourcePipelineReconciler) getComponentVersionAccess(ctx context.Context, obj *v1alpha1.ResourcePipeline) (ocmcore.ComponentVersionAccess, error) {
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

const (
	secretsInjectKey    = "$secrets"
	parametersInjectKey = "$parameters"
)

func parametersInjectFunc(parameters map[string]any, key string) (any, error) {
	if v, ok := parameters[key]; ok {
		return v, nil
	}

	return nil, fmt.Errorf("parameter with key %s not found", key)
}

func secretsInjectFunc(secrets map[string]any, key string) (any, error) {
	if v, ok := secrets[key]; ok {
		return v, nil
	}

	return nil, fmt.Errorf("secret with key %s not found", key)
}

func injectValue(secrets, parameters map[string]any, value string) (any, error) {
	idx := strings.Index(value, ".")
	if idx < 0 {
		return nil, fmt.Errorf("expected inject function name to be of format $func.key but was %s", value)
	}

	if idx+1 >= len(value) {
		return nil, fmt.Errorf("missing value from func key: %s", value)
	}

	funcName, key := value[:idx], value[idx+1:]

	switch {
	case strings.HasPrefix(funcName, secretsInjectKey):
		return secretsInjectFunc(secrets, key)
	case strings.HasPrefix(funcName, parametersInjectKey):
		return parametersInjectFunc(parameters, key)
	}

	return nil, fmt.Errorf("unknown inject function: %s", funcName)
}

// processValueFunctions takes a secret map and parameters map then starts updating the values
// based on placeholders in the values like `$secrets.kubeconfig` or `$parameters.replicas`.
func processValueFunctions(secretMap map[string]any, parameters map[string]any, values map[string]any) error {
	for k, v := range values {
		switch t := v.(type) {
		case string:
			if strings.HasPrefix(t, "$") {
				inject, err := injectValue(secretMap, parameters, t)
				if err != nil {
					return fmt.Errorf("failed to inject value: %w", err)
				}

				values[k] = inject
			}
		case map[string]any:
			if err := processValueFunctions(secretMap, parameters, t); err != nil {
				return fmt.Errorf("failed to process values: %w", err)
			}
		case []any:
			for i, e := range t {
				switch et := e.(type) {
				case string:
					if strings.HasPrefix(et, "$") {
						inject, err := injectValue(secretMap, parameters, et)
						if err != nil {
							return fmt.Errorf("failed to create inject value: %w", err)
						}

						t[i] = inject
					}
				case map[string]any:
					if err := processValueFunctions(secretMap, parameters, et); err != nil {
						return fmt.Errorf("failed to process values: %w", err)
					}
				}
			}
		}
	}

	return nil
}
