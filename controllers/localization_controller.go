// Copyright 2022.
// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/mandelsoft/vfs/pkg/osfs"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/open-component-model/ocm/pkg/contexts/ocm/utils/localize"
	"github.com/open-component-model/ocm/pkg/utils"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	deliveryv1alpha1 "github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/pkg/configdata"
	"github.com/open-component-model/ocm-controller/pkg/oci"
)

// LocalizationReconciler reconciles a Localization object
type LocalizationReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	ReconcileInterval time.Duration
	RetryInterval     time.Duration
	OCIRegistryAddr   string
}

//+kubebuilder:rbac:groups=delivery.ocm.software,resources=localizations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=delivery.ocm.software,resources=localizations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=delivery.ocm.software,resources=localizations/finalizers,verbs=update

// SetupWithManager sets up the controller with the Manager.
func (r *LocalizationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	snapshotSourceKey := ".metadata.snapshot.source"
	configKey := ".metadata.config"

	if err := mgr.GetCache().IndexField(context.TODO(), &deliveryv1alpha1.Localization{}, snapshotSourceKey,
		r.indexBy("Snapshot", "SourceRef")); err != nil {
		return fmt.Errorf("failed setting index fields: %w", err)
	}

	if err := mgr.GetCache().IndexField(context.TODO(), &deliveryv1alpha1.Localization{}, configKey,
		r.indexBy("ComponentDescriptor", "ConfigRef")); err != nil {
		return fmt.Errorf("failed setting index fields: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&deliveryv1alpha1.Localization{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(
			&source.Kind{Type: &deliveryv1alpha1.Snapshot{}},
			handler.EnqueueRequestsFromMapFunc(r.requestsForRevisionChangeOf(snapshotSourceKey)),
		).
		// Watches(
		//     &source.Kind{Type: &deliveryv1alpha1.ComponentDescriptor{}},
		//     handler.EnqueueRequestsFromMapFunc(r.requestsForRevisionChangeOf(configKey)),
		// ).
		Complete(r)
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *LocalizationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithName("localization-controller")

	obj := &v1alpha1.Localization{}
	if err := r.Client.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get localization object: %w", err)
	}

	log.Info("reconciling localization")

	return r.reconcile(ctx, obj)
}

func (r *LocalizationReconciler) reconcile(ctx context.Context, obj *v1alpha1.Localization) (ctrl.Result, error) {
	// get source snapshot
	srcSnapshot := &v1alpha1.Snapshot{}
	if err := r.Get(ctx, obj.GetSourceSnapshotKey(), srcSnapshot); err != nil {
		if apierrors.IsNotFound(err) {
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

	// read component version
	cv := types.NamespacedName{
		Name:      obj.Spec.ConfigRef.ComponentVersionRef.Name,
		Namespace: obj.Spec.ConfigRef.ComponentVersionRef.Namespace,
	}

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

	// get config resource
	configResource := componentDescriptor.GetResource(obj.Spec.ConfigRef.Resource.Name)
	config := configdata.ConfigData{}
	if err := GetResource(*srcSnapshot, &config); err != nil {
		return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()},
			fmt.Errorf("failed to get component resource: %w", err)
	}

	var localizations localize.Substitutions
	for _, l := range config.Localization {
		lr := componentDescriptor.GetResource(l.Resource.Name)
		if lr == nil {
			continue
		}

		access, err := GetImageReference(lr)
		if err != nil {
			return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()},
				fmt.Errorf("failed to get resource access: %w", err)
		}

		ref, err := name.ParseReference(access)
		if err != nil {
			return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()},
				fmt.Errorf("failed to get resource access: %w", err)
		}

		if l.Repository != "" {

			if err := localizations.Add("repository", l.File, l.Repository, ref.Context().Name()); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to add repository: %w", err)
			}
		}

		if l.Image != "" {
			if err := localizations.Add("image", l.File, l.Image, ref.Name()); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to add image ref name: %w", err)
			}
		}

		if l.Tag != "" {
			if err := localizations.Add("image", l.File, l.Tag, ref.Identifier()); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to add identifier: %w", err)
			}
		}
	}

	virtualFS, err := osfs.NewTempFileSystem()
	if err != nil {
		return ctrl.Result{RequeueAfter: r.RetryInterval}, fmt.Errorf("fs error: %w", err)
	}
	// defer vfs.Cleanup(virtualFS)

	if err := utils.ExtractTarToFs(virtualFS, bytes.NewBuffer(srcSnapshotData)); err != nil {
		return ctrl.Result{RequeueAfter: r.RetryInterval}, fmt.Errorf("extract tar error: %w", err)
	}

	if err := localize.Substitute(localizations, virtualFS); err != nil {
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
	defer os.Remove(artifactPath.Name())

	if err := BuildTar(artifactPath.Name(), sourceDir); err != nil {
		return ctrl.Result{RequeueAfter: r.RetryInterval}, fmt.Errorf("build tar error: %w", err)
	}

	// create snapshot
	snapshotName := fmt.Sprintf(
		"%s/%s/%s/%s/%s",
		r.OCIRegistryAddr,
		componentVersion.Spec.Component,
		componentVersion.Status.ReconciledVersion,
		obj.Spec.SnapshotTemplate.Name,
		obj.Spec.SnapshotTemplate.Tag,
	)
	snapshotDigest, err := r.writeSnapshot(snapshotName, artifactPath.Name())
	if err != nil {
		return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()}, err
	}

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
				return fmt.Errorf("failed to set owner reference on snapshot: %w", err)
			}
		}
		snapshotCR.Spec = v1alpha1.SnapshotSpec{
			Ref:    strings.TrimPrefix(snapshotName, r.OCIRegistryAddr+"/"),
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

func (r *LocalizationReconciler) requestsForRevisionChangeOf(indexKey string) func(obj client.Object) []reconcile.Request {
	return func(obj client.Object) []reconcile.Request {
		snap, ok := obj.(*v1alpha1.Snapshot)
		if !ok {
			panic(fmt.Sprintf("expected snapshot but got: %T", obj))
		}

		if snap.GetDigest() == "" {
			return nil
		}

		ctx := context.Background()
		var list v1alpha1.LocalizationList
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

func (r *LocalizationReconciler) indexBy(kind, field string) func(o client.Object) []string {
	return func(o client.Object) []string {
		l, ok := o.(*v1alpha1.Localization)
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

func (r *LocalizationReconciler) getSnapshotBytes(snapshot *deliveryv1alpha1.Snapshot) ([]byte, error) {
	image := strings.TrimPrefix(snapshot.Status.Image, "http://")
	image = strings.TrimPrefix(image, "https://")
	repo, err := oci.NewRepository(image, oci.WithInsecure())
	if err != nil {
		return nil, err
	}

	reader, err := repo.FetchBlob(snapshot.Spec.Digest)
	if err != nil {
		return nil, err
	}

	return io.ReadAll(reader)
}

func (r *LocalizationReconciler) writeSnapshot(snapshotName, artifactPath string) (string, error) {
	repo, err := oci.NewRepository(snapshotName, oci.WithInsecure())
	if err != nil {
		return "", fmt.Errorf("failed create new repository: %w", err)
	}

	file, err := os.Open(artifactPath)
	if err != nil {
		return "", fmt.Errorf("failed to open created archive: %w", err)
	}
	defer file.Close()
	digest, err := repo.PushStreamBlob(file, "")
	if err != nil {
		return "", fmt.Errorf("failed to push blob to local registry: %w", err)
	}

	return digest.Digest.String(), nil
}
