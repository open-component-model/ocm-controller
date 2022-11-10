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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	ociclient "github.com/fluxcd/pkg/oci/client"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	gcrv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/mandelsoft/vfs/pkg/osfs"
	"github.com/mandelsoft/vfs/pkg/vfs"

	deliveryv1alpha1 "github.com/open-component-model/ocm-controller/api/v1alpha1"
	v1alpha1 "github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/pkg/configdata"
	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/accessmethods/localblob"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/accessmethods/ociartefact"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/accessmethods/ociblob"
	ocmapi "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/ocm.software/v3alpha1"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/cpi"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/utils/localize"
	ocmruntime "github.com/open-component-model/ocm/pkg/runtime"
	"github.com/open-component-model/ocm/pkg/utils"
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

	// TODO: Change this to ComponentVersion
	// read component descriptor
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

	componentDescriptor, err := r.getComponentDescriptor(ctx, obj, componentVersion.Status.ComponentDescriptor)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get component descriptor from version")
	}
	if componentDescriptor == nil {
		return ctrl.Result{
			RequeueAfter: obj.GetRequeueAfter(),
		}, fmt.Errorf("couldn't find component descriptor for reference '%s' or any root components", obj.Spec.ConfigRef.ReferencePath)
	}

	// get config resource
	configResource := componentDescriptor.GetResource(obj.Spec.ConfigRef.Resource.Name)
	config := configdata.ConfigData{}
	if err := r.getResource(ctx, configResource, &config); err != nil {
		return ctrl.Result{RequeueAfter: obj.GetRequeueAfter()},
			fmt.Errorf("failed to get component resource: %w", err)
	}

	var localizations localize.Substitutions
	for _, l := range config.Localization {
		lr := componentDescriptor.GetResource(l.Resource.Name)
		if lr == nil {
			continue
		}

		access, err := r.getImageReference(lr)
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
			localizations.Add("repository", l.File, l.Repository, ref.Context().Name())
		}

		if l.Image != "" {
			localizations.Add("image", l.File, l.Image, ref.Name())
		}

		if l.Tag != "" {
			localizations.Add("image", l.File, l.Tag, ref.Identifier())
		}
	}

	virtualFS, err := osfs.NewTempFileSystem()
	if err != nil {
		return ctrl.Result{RequeueAfter: r.RetryInterval}, fmt.Errorf("fs error: %w", err)
	}
	defer vfs.Cleanup(virtualFS)

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
	defer func() {
		if err != nil {
			os.Remove(artifactPath.Name())
		}
	}()

	if err := r.buildTar(artifactPath.Name(), sourceDir, virtualFS); err != nil {
		return ctrl.Result{RequeueAfter: r.RetryInterval}, fmt.Errorf("build tar error: %w", err)
	}

	// create snapshot
	snapshotName := fmt.Sprintf("%s/snapshots/%s:%s", r.OCIRegistryAddr, obj.Spec.SnapshotTemplate.Name, obj.Spec.SnapshotTemplate.Tag)
	snapshotDigest, err := r.writeSnapshot(ctx, snapshotName, artifactPath.Name())
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
			controllerutil.SetOwnerReference(obj, snapshotCR, r.Scheme)
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

func (r *LocalizationReconciler) getResourceAccess(resource *ocmapi.Resource) (cpi.AccessSpec, error) {
	var accessSpec cpi.AccessSpec
	rawAccessSpec, err := resource.Access.GetRaw()
	if err != nil {
		return nil, err
	}

	switch resource.Access.Type {
	case "localBlob":
		accessSpec = &localblob.AccessSpec{}
	case "ociblob":
		accessSpec = &ociblob.AccessSpec{}
	case "ociArtefact":
		accessSpec = &ociartefact.AccessSpec{}
	}

	if err := ocmruntime.DefaultJSONEncoding.Unmarshal(rawAccessSpec, accessSpec); err != nil {
		return nil, fmt.Errorf("failed to unmarshal access spec: %w", err)
	}

	return accessSpec, err
}

func (r *LocalizationReconciler) getImageReference(resource *ocmapi.Resource) (string, error) {
	accessSpec, err := r.getResourceAccess(resource)
	if err != nil {
		return "", err
	}

	switch resource.Access.Type {
	case "localBlob":
		gs, err := accessSpec.(*localblob.AccessSpec).GlobalAccess.Evaluate(ocm.DefaultContext())
		if err != nil {
			return "", err
		}
		ref := gs.(*ociblob.AccessSpec).Reference
		sha := gs.(*ociblob.AccessSpec).Digest.String()
		return fmt.Sprintf("%s:%s@%s", ref, resource.GetVersion(), sha), nil
	case "ociblob":
		return accessSpec.(*ociblob.AccessSpec).Reference, nil
	case "ociArtefact":
		return accessSpec.(*ociartefact.AccessSpec).ImageReference, nil
	}

	return "", errors.New("could not get access information")
}

func (r *LocalizationReconciler) getResource(ctx context.Context, resource *ocmapi.Resource, result interface{}) error {
	access, err := r.getImageReference(resource)
	if err != nil {
		return fmt.Errorf("failed to create digest: %w", err)
	}

	digest, err := name.NewDigest(access, name.Insecure)
	if err != nil {
		return fmt.Errorf("failed to create digest: %w", err)
	}

	// proxy image requests via the in-cluster oci-registry
	proxyURL, err := url.Parse(fmt.Sprintf("http://%s", r.OCIRegistryAddr))
	if err != nil {
		return fmt.Errorf("failed to parse oci registry url: %w", err)
	}

	// create a transport to the in-cluster oci-registry
	tr := newCustomTransport(remote.DefaultTransport.(*http.Transport).Clone(), proxyURL)

	// set context values to be transmitted as headers on the registry requests
	for k, v := range map[string]string{
		"registry":   digest.Repository.Registry.String(),
		"repository": digest.Repository.String(),
		"digest":     digest.String(),
		"image":      digest.Name(),
		"tag":        resource.Version,
	} {
		ctx = context.WithValue(ctx, contextKey(k), v)
	}

	// fetch the layer
	remoteLayer, err := remote.Layer(digest, remote.WithTransport(tr), remote.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("failed to retrieve layer: %w", err)
	}

	data, err := remoteLayer.Uncompressed()
	if err != nil {
		return fmt.Errorf("failed to read layer: %w", err)
	}

	configBytes, err := io.ReadAll(data)
	if err != nil {
		return fmt.Errorf("failed to read layer: %w", err)
	}

	return ocmruntime.DefaultYAMLEncoding.Unmarshal(configBytes, result)
}

// modified version of https://github.com/fluxcd/pkg/blob/2ee90dd5b2ec033f44881f160e29584cceda8f37/oci/client/build.go
func (r *LocalizationReconciler) buildTar(artifactPath, sourceDir string, virtualFs vfs.FileSystem) error {
	if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
		return fmt.Errorf("invalid source dir path: %s", sourceDir)
	}

	tf, err := os.CreateTemp("", "")
	if err != nil {
		return err
	}
	tmpName := tf.Name()
	defer func() {
		if err != nil {
			os.Remove(tmpName)
		}
	}()

	gw := gzip.NewWriter(tf)
	tw := tar.NewWriter(gw)

	if err := filepath.Walk(sourceDir, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Ignore anything that is not a file or directories e.g. symlinks
		if m := fi.Mode(); !(m.IsRegular() || m.IsDir()) {
			return nil
		}

		header, err := tar.FileInfoHeader(fi, p)
		if err != nil {
			return err
		}
		// The name needs to be modified to maintain directory structure
		// as tar.FileInfoHeader only has access to the base name of the file.
		// Ref: https://golang.org/src/archive/tar/common.go?#L626
		relFilePath := p
		if filepath.IsAbs(sourceDir) {
			relFilePath, err = filepath.Rel(sourceDir, p)
			if err != nil {
				return err
			}
		}
		header.Name = relFilePath

		// Remove any environment specific data.
		header.Gid = 0
		header.Uid = 0
		header.Uname = ""
		header.Gname = ""
		header.ModTime = time.Time{}
		header.AccessTime = time.Time{}
		header.ChangeTime = time.Time{}

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !fi.Mode().IsRegular() {
			return nil
		}
		f, err := os.Open(p)
		if err != nil {
			f.Close()
			return err
		}
		if _, err := io.Copy(tw, f); err != nil {
			f.Close()
			return err
		}
		return f.Close()
	}); err != nil {
		tw.Close()
		gw.Close()
		tf.Close()
		return err
	}

	if err := tw.Close(); err != nil {
		gw.Close()
		tf.Close()
		return err
	}
	if err := gw.Close(); err != nil {
		tf.Close()
		return err
	}
	if err := tf.Close(); err != nil {
		return err
	}

	if err := os.Chmod(tmpName, 0o640); err != nil {
		return err
	}

	return os.Rename(tmpName, artifactPath)
}

func (r *LocalizationReconciler) writeSnapshot(ctx context.Context, snapshotName, artifactPath string) (string, error) {
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

func (r *LocalizationReconciler) getComponentDescriptorObject(ctx context.Context, ref meta.NamespacedObjectReference) (*v1alpha1.ComponentDescriptor, error) {
	componentDescriptor := &v1alpha1.ComponentDescriptor{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      ref.Name,
		Namespace: ref.Namespace,
	}, componentDescriptor); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find component descriptor: %w", err)
	}
	return componentDescriptor, nil
}

func (r *LocalizationReconciler) getComponentDescriptor(ctx context.Context, localization *v1alpha1.Localization, obj v1alpha1.Reference) (*v1alpha1.ComponentDescriptor, error) {
	// Return early if there was no name defined.
	if localization.Spec.ConfigRef.ReferencePath == "" {
		return r.getComponentDescriptorObject(ctx, obj.ComponentDescriptorRef)
	}

	// Handle the nested loop. If we get to this part, we check if the reference that we found
	// is the one we were looking for.
	// TODO: What about extra identity?
	if obj.Name == localization.Spec.ConfigRef.ReferencePath {
		return r.getComponentDescriptorObject(ctx, obj.ComponentDescriptorRef)
	}

	// This is not the reference object we are looking for, let's dig deeper.
	for _, ref := range obj.References {
		desc, err := r.getComponentDescriptor(ctx, localization, ref)
		if err != nil {
			return nil, err
		}
		// recursive call for ref did not result in a reference
		// get the next ref, do the same lookup again
		if desc == nil {
			continue
		}

		return desc, nil
	}

	return nil, nil
}
