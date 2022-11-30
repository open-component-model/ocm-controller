// Copyright 2022.
// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	ociclient "github.com/fluxcd/pkg/oci/client"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	gcrv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/mandelsoft/spiff/spiffing"
	"github.com/mandelsoft/vfs/pkg/osfs"
	"github.com/mandelsoft/vfs/pkg/vfs"
	deliveryv1alpha1 "github.com/open-component-model/ocm-controller/api/v1alpha1"
	v1alpha1 "github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/pkg/configdata"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/utils/localize"
	"github.com/open-component-model/ocm/pkg/errors"
	ocmruntime "github.com/open-component-model/ocm/pkg/runtime"
	"github.com/open-component-model/ocm/pkg/spiff"
	"github.com/open-component-model/ocm/pkg/utils"
)

// ConfigurationReconciler reconciles a Configuration object
type ConfigurationReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	ReconcileInterval time.Duration
	RetryInterval     time.Duration
	OCIRegistryAddr   string
}

//+kubebuilder:rbac:groups=delivery.ocm.software,resources=configurations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=delivery.ocm.software,resources=configurations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=delivery.ocm.software,resources=configurations/finalizers,verbs=update

// SetupWithManager sets up the controller with the Manager.
func (r *ConfigurationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	snapshotSourceKey := ".metadata.snapshot.source"
	configKey := ".metadata.config"

	if err := mgr.GetCache().IndexField(context.TODO(), &deliveryv1alpha1.Configuration{}, snapshotSourceKey,
		r.indexBy("Snapshot", "SourceRef")); err != nil {
		return fmt.Errorf("failed setting index fields: %w", err)
	}

	if err := mgr.GetCache().IndexField(context.TODO(), &deliveryv1alpha1.Configuration{}, configKey,
		r.indexBy("ComponentDescriptor", "ConfigRef")); err != nil {
		return fmt.Errorf("failed setting index fields: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&deliveryv1alpha1.Configuration{}).
		Watches(
			&source.Kind{Type: &deliveryv1alpha1.Snapshot{}},
			handler.EnqueueRequestsFromMapFunc(r.requestsForRevisionChangeOf(snapshotSourceKey)),
		).
		Complete(r)
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ConfigurationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithName("configuration-controller")

	obj := &v1alpha1.Configuration{}
	if err := r.Client.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, fmt.Errorf("failed to get configuration object: %w", err)
	}

	log.Info("reconciling configuration")

	return r.reconcile(ctx, obj)
}

func (r *ConfigurationReconciler) reconcile(ctx context.Context, obj *v1alpha1.Configuration) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithName("configuration-controller")
	// get source snapshot
	srcSnapshot := &v1alpha1.Snapshot{}
	if err := r.Get(ctx, obj.GetSourceSnapshotKey(), srcSnapshot); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("snapshot not found")
			return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, nil
		}
		return ctrl.Result{RequeueAfter: r.RetryInterval},
			fmt.Errorf("failed to get component object: %w", err)
	}

	srcSnapshotData, err := r.getSnapshotBytes(srcSnapshot)
	if err != nil {
		return ctrl.Result{RequeueAfter: r.RetryInterval}, err
	}

	// Initialize the patch helper.
	patchHelper, err := patch.NewHelper(obj, r.Client)
	if err != nil {
		return ctrl.Result{RequeueAfter: r.RetryInterval}, err
	}

	cv := types.NamespacedName{
		Name:      obj.Spec.ConfigRef.ComponentVersionRef.Name,
		Namespace: obj.Spec.ConfigRef.ComponentVersionRef.Namespace,
	}

	//TODO@souleb: index component descriptor by component version
	// https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/client#FieldIndexer
	// this will allow us to avoid fetching the component version to get the component descriptor
	// Example:
	// if err := mgr.GetCache().IndexField(context.TODO(), &deliveryv1alpha1.ComponentDescriptor{}, componentVersionKey, func(rawObj client.Object) []string {
	// 	cd := rawObj.(*deliveryv1alpha1.ComponentDescriptor)
	// 	owner := metav1.GetControllerOf(cd)
	// 	if owner == nil {
	// 		return nil
	// 	}
	// 	if owner.APIVersion != deliveryv1alpha1.GroupVersion.String() || owner.Kind != "ComponentVersion" {
	// 		return nil
	// 	}
	// 	return []string{owner.Name}
	// }); err != nil {	return fmt.Errorf("failed setting index fields: %w", err) }
	//}
	// we can then use the following to get the component descriptor
	// var cd v1alpha1.ComponentDescriptorList
	// if err := r.List(ctx, &cd, client.InNamespace(obj.Spec.ConfigRef.ComponentVersionRef.Namespace), client.MatchingFields{descriptorOwnerKey: obj.Spec.ConfigRef.ComponentVersionRef.Name}); err != nil {
	// 	return ctrl.Result{RequeueAfter: r.RetryInterval}, err
	// }
	componentVersion := &v1alpha1.ComponentVersion{}
	if err := r.Get(ctx, cv, componentVersion); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, nil
		}

		return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()},
			fmt.Errorf("failed to get component object: %w", err)
	}

	componentDescriptor, err := GetComponentDescriptor(ctx, r.Client, obj.Spec.ConfigRef.Resource.ReferencePath, componentVersion.Status.ComponentDescriptor)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get component descriptor from version")
	}
	if componentDescriptor == nil {
		return ctrl.Result{
			RequeueAfter: obj.GetRequeueAfter(),
		}, fmt.Errorf("couldn't find component descriptor for reference '%s' or any root components", obj.Spec.ConfigRef.Resource.ReferencePath)
	}

	// TO DO@souleb: guard against nil return value
	configResource := componentDescriptor.GetResource(obj.Spec.ConfigRef.Resource.Name)
	if configResource == nil {
		return ctrl.Result{
			RequeueAfter: obj.GetRequeueAfter(),
		}, fmt.Errorf("couldn't find config resource for resource name '%s'", obj.Spec.ConfigRef.Resource.Name)
	}
	config := configdata.ConfigData{}
	if err := GetResource(ctx, *srcSnapshot, configResource.Version, r.OCIRegistryAddr, &config); err != nil {
		return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()},
			fmt.Errorf("failed to get component resource: %w", err)
	}

	var rules localize.Substitutions
	for i, l := range config.Configuration.Rules {
		rules.Add(fmt.Sprintf("subst-%d", i), l.File, l.Path, l.Value)
	}

	defaults, err := json.Marshal(config.Configuration.Defaults)
	if err != nil {
		return ctrl.Result{RequeueAfter: r.RetryInterval}, fmt.Errorf("configurator error: %w", err)
	}

	values, err := json.Marshal(obj.Spec.Values)
	if err != nil {
		return ctrl.Result{RequeueAfter: r.RetryInterval}, fmt.Errorf("configurator error: %w", err)
	}

	schema, err := json.Marshal(config.Configuration.Schema)
	if err != nil {
		return ctrl.Result{RequeueAfter: r.RetryInterval}, fmt.Errorf("configurator error: %w", err)
	}

	configSubstitions, err := r.configurator(rules, defaults, values, schema)
	if err != nil {
		return ctrl.Result{RequeueAfter: r.RetryInterval}, fmt.Errorf("configurator error: %w", err)
	}

	virtualFS, err := osfs.NewTempFileSystem()
	if err != nil {
		return ctrl.Result{RequeueAfter: r.RetryInterval}, fmt.Errorf("fs error: %w", err)
	}
	defer vfs.Cleanup(virtualFS)

	if err := utils.ExtractTarToFs(virtualFS, bytes.NewBuffer(srcSnapshotData)); err != nil {
		return ctrl.Result{RequeueAfter: r.RetryInterval}, fmt.Errorf("extract tar error: %w", err)
	}

	if err := localize.Substitute(configSubstitions, virtualFS); err != nil {
		return ctrl.Result{RequeueAfter: r.RetryInterval}, fmt.Errorf("extract tar error: %w", err)
	}

	fi, err := virtualFS.Stat("/")
	if err != nil {
		return ctrl.Result{RequeueAfter: r.RetryInterval}, fmt.Errorf("fs error: %w", err)
	}

	sourceDir := filepath.Join(os.TempDir(), fi.Name())

	artifactPath, err := os.CreateTemp("", "snapshot-artifact-*.tgz")
	if err != nil {
		return ctrl.Result{RequeueAfter: r.RetryInterval}, fmt.Errorf("fs error: %w", err)
	}
	defer func() {
		if err != nil {
			os.Remove(artifactPath.Name())
		}
	}()

	if err := BuildTar(artifactPath.Name(), sourceDir); err != nil {
		return ctrl.Result{RequeueAfter: r.RetryInterval}, fmt.Errorf("build tar error: %w", err)
	}

	// create snapshot
	snapshotName := fmt.Sprintf("%s/snapshots/%s:%s", r.OCIRegistryAddr, obj.Spec.SnapshotTemplate.Name, obj.Spec.SnapshotTemplate.Tag)
	snapshotDigest, err := r.writeSnapshot(ctx, snapshotName, artifactPath.Name())
	if err != nil {
		return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, err
	}

	//TODO@soule: start by checking config generation change
	// then after computing snapshot, check if snapshot changed based on config.LastSnapshotDigest
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
				return fmt.Errorf("failed to set owner reference: %w", err)
			}
		}
		snapshotCR.Spec = v1alpha1.SnapshotSpec{
			Ref:    strings.TrimPrefix(snapshotName, r.OCIRegistryAddr+"/snapshots/"),
			Digest: snapshotDigest,
		}
		return nil
	})

	if err != nil {
		return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()},
			fmt.Errorf("failed to create or update component descriptor: %w", err)
	}

	obj.Status.LatestSnapshotDigest = srcSnapshot.GetDigest()
	obj.Status.LatestConfigVersion = fmt.Sprintf("%s:%s", configResource.Name, configResource.Version)
	obj.Status.ObservedGeneration = obj.GetGeneration()

	if err := patchHelper.Patch(ctx, obj); err != nil {
		return ctrl.Result{
			RequeueAfter: obj.GetRequeueAfter(),
		}, fmt.Errorf("failed to patch resource and set snaphost value: %w", err)
	}

	return ctrl.Result{RequeueAfter: r.ReconcileInterval}, nil
}

func (r *ConfigurationReconciler) requestsForRevisionChangeOf(indexKey string) func(obj client.Object) []reconcile.Request {
	return func(obj client.Object) []reconcile.Request {
		snap, ok := obj.(*v1alpha1.Snapshot)
		if !ok {
			panic(fmt.Sprintf("expected snapshot but got: %T", obj))
		}

		if snap.GetDigest() == "" {
			return nil
		}

		ctx := context.Background()
		var list v1alpha1.ConfigurationList
		if err := r.List(ctx, &list, client.MatchingFields{
			indexKey: client.ObjectKeyFromObject(obj).String(),
		}); err != nil {
			return nil
		}

		var reqs []reconcile.Request
		for _, d := range list.Items {
			if snap.GetDigest() == d.Status.LatestSnapshotDigest {
				continue
			}
			reqs = append(reqs, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: d.Namespace,
					Name:      d.Name,
				},
			})
		}

		return reqs
	}
}

func (r *ConfigurationReconciler) indexBy(kind, field string) func(o client.Object) []string {
	return func(o client.Object) []string {
		l, ok := o.(*v1alpha1.Configuration)
		if !ok {
			panic(fmt.Sprintf("Expected a Localization, got %T", o))
		}

		switch field {
		case "SourceRef":
			if l.Spec.SourceRef.Kind == kind {
				namespace := l.GetNamespace()
				if l.Spec.SourceRef.Namespace != "" {
					namespace = l.Spec.SourceRef.Namespace
				}
				return []string{fmt.Sprintf("%s/%s", namespace, l.Spec.SourceRef.Name)}
			}
		case "ConfigRef":
			namespace := l.GetNamespace()
			if l.Spec.ConfigRef.ComponentVersionRef.Namespace != "" {
				namespace = l.Spec.ConfigRef.ComponentVersionRef.Namespace
			}
			return []string{fmt.Sprintf("%s/%s", namespace, strings.ReplaceAll(l.Spec.ConfigRef.ComponentVersionRef.Name, "/", "-"))}
		default:
			return nil
		}

		return nil
	}
}

func (r *ConfigurationReconciler) getSnapshotBytes(snapshot *deliveryv1alpha1.Snapshot) ([]byte, error) {
	digest, err := name.NewDigest(snapshot.GetBlob(), name.Insecure)
	if err != nil {
		return nil, err
	}

	remoteLayer, err := remote.Layer(digest)
	if err != nil {
		return nil, err
	}

	layerData, err := remoteLayer.Uncompressed()
	if err != nil {
		return nil, err
	}

	return io.ReadAll(layerData)
}

func (r *ConfigurationReconciler) writeSnapshot(ctx context.Context, snapshotName, artifactPath string) (string, error) {
	ref, err := name.ParseReference(snapshotName, name.Insecure)
	if err != nil {
		return "", fmt.Errorf("failed to create snapshot reference: %w", err)
	}

	ct := time.Now()
	snapshotMeta := ociclient.Metadata{
		Created: ct.Format(time.RFC3339),
	}

	// add metadata
	snapshot, err := crane.Append(empty.Image, artifactPath)
	if err != nil {
		return "", fmt.Errorf("failed to create snapshot image: %w", err)
	}

	snapshot = mutate.Annotations(snapshot, snapshotMeta.ToAnnotations()).(gcrv1.Image)

	// write snapshot to registry
	if err := remote.Write(ref, snapshot); err != nil {
		return "", fmt.Errorf("failed to write snapshot: %w", err)
	}

	layers, err := snapshot.Layers()
	if err != nil {
		return "", fmt.Errorf("failed to get snapshot layers: %w", err)
	}

	digest, err := layers[0].Digest()
	if err != nil {
		return "", fmt.Errorf("failed to get snapshot digest: %w", err)
	}

	return digest.String(), nil
}

func (r *ConfigurationReconciler) configurator(subst []localize.Substitution, defaults, values, schema []byte) (localize.Substitutions, error) {
	// configure defaults
	templ := make(map[string]any)
	if err := ocmruntime.DefaultYAMLEncoding.Unmarshal(defaults, &templ); err != nil {
		return nil, errors.Wrapf(err, "cannot unmarshal template")
	}

	// configure values overrides... must be a better way
	var valuesMap map[string]any
	if err := ocmruntime.DefaultYAMLEncoding.Unmarshal(values, &valuesMap); err != nil {
		return nil, errors.Wrapf(err, "cannot unmarshal template")
	}

	for k, v := range valuesMap {
		if _, ok := templ[k]; ok {
			templ[k] = v
		}
	}

	// configure adjustments
	list := []any{}
	for _, e := range subst {
		list = append(list, e)
	}

	templ["adjustments"] = list

	templateBytes, err := ocmruntime.DefaultJSONEncoding.Marshal(templ)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot marshal template")
	}

	if len(schema) > 0 {
		if err := spiff.ValidateByScheme(values, schema); err != nil {
			return nil, errors.Wrapf(err, "validation failed")
		}
	}

	config, err := spiff.CascadeWith(spiff.TemplateData("adjustments", templateBytes), spiff.Mode(spiffing.MODE_PRIVATE))
	if err != nil {
		return nil, errors.Wrapf(err, "error processing template")
	}

	var result struct {
		Adjustments localize.Substitutions `json:"adjustments,omitempty"`
	}

	if err := ocmruntime.DefaultYAMLEncoding.Unmarshal(config, &result); err != nil {
		return nil, errors.Wrapf(err, "error processing template")
	}

	return result.Adjustments, nil
}
