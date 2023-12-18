// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"github.com/containers/image/v5/pkg/compression"
	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/http/fetch"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/tar"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/mandelsoft/spiff/spiffing"
	"github.com/mandelsoft/vfs/pkg/osfs"
	"github.com/open-component-model/ocm-controller/pkg/snapshot"
	ocmcore "github.com/open-component-model/ocm/pkg/contexts/ocm"
	"github.com/open-component-model/ocm/pkg/utils/tarutils"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/pkg/cache"
	"github.com/open-component-model/ocm-controller/pkg/component"
	"github.com/open-component-model/ocm-controller/pkg/configdata"
	"github.com/open-component-model/ocm-controller/pkg/ocm"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/accessmethods/localblob"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/accessmethods/ociartifact"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/accessmethods/ociblob"
	ocmmetav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	utils2 "github.com/open-component-model/ocm/pkg/contexts/ocm/utils"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/utils/localize"
	ocmruntime "github.com/open-component-model/ocm/pkg/runtime"
	"github.com/open-component-model/ocm/pkg/spiff"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// errTar defines an error that occurs when the resource is not a tar archive.
var errTar = errors.New("expected tarred directory content for configuration/localization resources, got plain text")

// MutationReconcileLooper holds dependencies required to reconcile a mutation object.
type MutationReconcileLooper struct {
	Scheme         *runtime.Scheme
	OCMClient      ocm.Contract
	Client         client.Client
	Cache          cache.Cache
	DynamicClient  dynamic.Interface
	SnapshotWriter snapshot.Writer
}

// ReconcileMutationObject reconciles mutation objects and writes a snapshot to the cache.
func (m *MutationReconcileLooper) ReconcileMutationObject(ctx context.Context, obj v1alpha1.MutationObject) (int64, error) {
	mutationSpec := obj.GetSpec()

	sourceData, err := m.getData(ctx, &mutationSpec.SourceRef)
	if err != nil {
		return -1, fmt.Errorf("failed to get data for source ref: %w", err)
	}

	sourceID, err := m.getIdentity(ctx, &mutationSpec.SourceRef)
	if err != nil {
		return -1, fmt.Errorf("failed to get identity for source ref: %w", err)
	}

	obj.GetStatus().LatestSourceVersion = sourceID[v1alpha1.ComponentVersionKey]

	if len(sourceData) == 0 {
		return -1, fmt.Errorf("source resource data cannot be empty")
	}

	sourceDir, snapshotID, err := m.performMutation(ctx, obj, mutationSpec, sourceData)
	if err != nil {
		return -1, err
	}

	defer os.RemoveAll(sourceDir)

	digest, size, err := m.SnapshotWriter.Write(ctx, obj, sourceDir, snapshotID)
	if err != nil {
		return -1, fmt.Errorf("error writing snapshot: %w", err)
	}

	obj.GetStatus().LatestSnapshotDigest = digest

	return size, nil
}

func (m *MutationReconcileLooper) performMutation(
	ctx context.Context,
	obj v1alpha1.MutationObject,
	mutationSpec *v1alpha1.MutationSpec,
	sourceData []byte,
) (string, ocmmetav1.Identity, error) {
	var (
		snapshotID ocmmetav1.Identity
		sourceDir  string
	)

	if mutationSpec.ConfigRef != nil {
		configData, err := m.getData(ctx, mutationSpec.ConfigRef)
		if err != nil {
			return "", ocmmetav1.Identity{}, fmt.Errorf("failed to get data for config ref: %w", err)
		}

		snapshotID, err = m.getIdentity(ctx, mutationSpec.ConfigRef)
		if err != nil {
			return "", ocmmetav1.Identity{}, fmt.Errorf("failed to get identity for config ref: %w", err)
		}

		obj.GetStatus().LatestConfigVersion = snapshotID[v1alpha1.ComponentVersionKey]

		sourceDir, err = m.mutate(ctx, mutationSpec, sourceData, configData, obj.GetNamespace())
		if err != nil {
			return "", ocmmetav1.Identity{}, err
		}
	}

	if mutationSpec.PatchStrategicMerge != nil {
		tmpDir, err := os.MkdirTemp("", "kustomization-")
		if err != nil {
			err = fmt.Errorf("tmp dir error: %w", err)

			return "", ocmmetav1.Identity{}, err
		}
		defer os.RemoveAll(tmpDir)

		gitSource, err := m.getSource(ctx, mutationSpec.PatchStrategicMerge.Source.SourceRef)
		if err != nil {
			return "", ocmmetav1.Identity{}, fmt.Errorf("failed to get patch source: %w", err)
		}

		obj.GetStatus().LatestPatchSourceVersion = gitSource.GetArtifact().Revision

		sourcePath := mutationSpec.PatchStrategicMerge.Source.Path
		targetPath := mutationSpec.PatchStrategicMerge.Target.Path

		sourceDir, snapshotID, err = m.strategicMergePatch(gitSource, sourceData, tmpDir, sourcePath, targetPath)
		if err != nil {
			return "", ocmmetav1.Identity{}, fmt.Errorf("failed to perform strategic merge patch: %w", err)
		}
	}

	return sourceDir, snapshotID, nil
}

func (m *MutationReconcileLooper) configure(
	ctx context.Context,
	data, configObj []byte,
	mutationSpec *v1alpha1.MutationSpec,
	namespace string,
) (string, error) {
	configValues, err := m.getValues(ctx, mutationSpec, namespace)
	if err != nil {
		return "", fmt.Errorf("failed to get values: %w", err)
	}

	log := log.FromContext(ctx)

	virtualFS, err := osfs.NewTempFileSystem()
	if err != nil {
		return "", fmt.Errorf("fs error: %w", err)
	}

	fi, err := virtualFS.Stat("/")
	if err != nil {
		return "", fmt.Errorf("fs error: %w", err)
	}

	sourceDir := filepath.Join(os.TempDir(), fi.Name())

	if !isTar(data) {
		return "", errTar
	}

	if err := tarutils.ExtractTarToFs(virtualFS, bytes.NewBuffer(data)); err != nil {
		return "", fmt.Errorf("extract tar error: %w", err)
	}

	rules, err := m.createSubstitutionRulesForConfigurationValues(configObj, configValues)
	if err != nil {
		return "", err
	}

	if len(rules) == 0 {
		log.Info("no rules generated from the available config data; the generate snapshot will have no modifications")
	}

	if err := localize.Substitute(rules, virtualFS); err != nil {
		return "", fmt.Errorf("localization substitution failed: %w", err)
	}

	return sourceDir, nil
}

func (m *MutationReconcileLooper) localize(
	ctx context.Context,
	mutationSpec *v1alpha1.MutationSpec,
	data, configObj []byte,
) (string, error) {
	log := log.FromContext(ctx)

	cv, err := m.getComponentVersion(ctx, mutationSpec.ConfigRef)
	if err != nil {
		return "", fmt.Errorf("failed to get component version: %w", err)
	}

	refPath := mutationSpec.ConfigRef.ResourceRef.ReferencePath

	virtualFS, err := osfs.NewTempFileSystem()
	if err != nil {
		return "", fmt.Errorf("fs error: %w", err)
	}

	fi, err := virtualFS.Stat("/")
	if err != nil {
		return "", fmt.Errorf("fs error: %w", err)
	}

	sourceDir := filepath.Join(os.TempDir(), fi.Name())

	if !isTar(data) {
		return "", errTar
	}

	if err := tarutils.ExtractTarToFs(virtualFS, bytes.NewBuffer(data)); err != nil {
		return "", fmt.Errorf("extract tar error: %w", err)
	}

	rules, err := m.createSubstitutionRulesForLocalization(ctx, cv, configObj, refPath)
	if err != nil {
		return "", fmt.Errorf("failed to create substitution rules for localization: %w", err)
	}

	if len(rules) == 0 {
		log.Info("no rules generated from the available config data; the generate snapshot will have no modifications")
	}

	if err := localize.Substitute(rules, virtualFS); err != nil {
		return "", fmt.Errorf("localization substitution failed: %w", err)
	}

	return sourceDir, nil
}

func (m *MutationReconcileLooper) fetchDataFromObjectReference(ctx context.Context, obj *v1alpha1.ObjectReference) ([]byte, error) {
	logger := log.FromContext(ctx)

	gvr := obj.GetGVR()
	src, err := m.DynamicClient.Resource(gvr).Namespace(obj.Namespace).Get(ctx, obj.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	snapshotName, ok, err := unstructured.NestedString(src.Object, "status", "snapshotName")
	if err != nil {
		return nil, fmt.Errorf("failed get the get snapshot: %w", err)
	}
	if !ok {
		return nil, errors.New("snapshot name not found in status")
	}

	key := types.NamespacedName{
		Name:      snapshotName,
		Namespace: obj.Namespace,
	}

	snapshot := &v1alpha1.Snapshot{}
	if err := m.Client.Get(ctx, key, snapshot); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("snapshot doesn't exist", "snapshot", key)

			return nil, err
		}

		return nil,
			fmt.Errorf("failed to get component object: %w", err)
	}

	if conditions.IsFalse(snapshot, meta.ReadyCondition) {
		return nil, fmt.Errorf("snapshot not ready: %s", key)
	}

	snapshotData, err := m.getSnapshotBytes(ctx, snapshot)
	if err != nil {
		return nil, err
	}

	return snapshotData, nil
}

func (m *MutationReconcileLooper) fetchDataFromComponentVersion(ctx context.Context, obj *v1alpha1.ObjectReference) ([]byte, error) {
	key := types.NamespacedName{
		Name:      obj.Name,
		Namespace: obj.Namespace,
	}

	componentVersion := &v1alpha1.ComponentVersion{}
	if err := m.Client.Get(ctx, key, componentVersion); err != nil {
		return nil, err
	}

	octx, err := m.OCMClient.CreateAuthenticatedOCMContext(ctx, componentVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to create authenticated client: %w", err)
	}

	resource, _, _, err := m.OCMClient.GetResource(ctx, octx, componentVersion, obj.ResourceRef)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch resource from component version: %w", err)
	}
	defer resource.Close()

	uncompressed, _, err := compression.AutoDecompress(resource)
	if err != nil {
		return nil, fmt.Errorf("failed to auto decompress: %w", err)
	}
	defer uncompressed.Close()

	// This will be problematic with a 6 Gig large object when it's trying to read it all.
	content, err := io.ReadAll(uncompressed)
	if err != nil {
		return nil, fmt.Errorf("failed to read resource data: %w", err)
	}

	return content, nil
}

// This might be problematic if the resource is too large in the snapshot. ReadAll will read it into memory.
func (m *MutationReconcileLooper) getSnapshotBytes(ctx context.Context, snapshot *v1alpha1.Snapshot) ([]byte, error) {
	name, err := ocm.ConstructRepositoryName(snapshot.Spec.Identity)
	if err != nil {
		return nil, fmt.Errorf("failed to construct name: %w", err)
	}

	reader, err := m.Cache.FetchDataByDigest(ctx, name, snapshot.Status.LastReconciledDigest)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch data: %w", err)
	}

	uncompressed, _, err := compression.AutoDecompress(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to auto decompress: %w", err)
	}
	defer uncompressed.Close()

	// We don't decompress snapshots because those are archives and are decompressed by the caching layer already.
	return io.ReadAll(uncompressed)
}

func (m *MutationReconcileLooper) createSubstitutionRulesForLocalization(
	ctx context.Context,
	cv *v1alpha1.ComponentVersion,
	data []byte,
	refPath []ocmmetav1.Identity,
) (localize.Substitutions, error) {
	config := &configdata.ConfigData{}
	if err := ocmruntime.DefaultYAMLEncoding.Unmarshal(data, config); err != nil {
		return nil,
			fmt.Errorf("failed to unmarshal content: %w", err)
	}

	octx, err := m.OCMClient.CreateAuthenticatedOCMContext(ctx, cv)
	if err != nil {
		return nil, fmt.Errorf("failed to create authenticated client: %w", err)
	}

	compvers, err := m.OCMClient.GetComponentVersion(ctx, octx, cv, cv.Spec.Component, cv.Status.ReconciledVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to get component version: %w", err)
	}
	defer compvers.Close()

	var localizations localize.Substitutions
	for _, l := range config.Localization {
		l := l
		if l.Mapping != nil {
			res, err := m.compileMapping(ctx, cv, l.Mapping.Transform)
			if err != nil {
				return nil, fmt.Errorf("failed to compile mapping: %w", err)
			}

			if err := localizations.Add("custom", l.File, l.Mapping.Path, res); err != nil {
				return nil, fmt.Errorf("failed to add identifier: %w", err)
			}

			continue
		}

		if err := m.performLocalization(octx, l, &localizations, refPath, compvers); err != nil {
			return nil, fmt.Errorf("failed to perform localization: %w", err)
		}
	}

	return localizations, nil
}

func (m *MutationReconcileLooper) performLocalization(
	octx ocmcore.Context,
	l configdata.LocalizationRule,
	localizations *localize.Substitutions,
	refPath []ocmmetav1.Identity,
	compvers ocmcore.ComponentVersionAccess,
) error {
	resourceRef := ocmmetav1.NewNestedResourceRef(ocmmetav1.NewIdentity(l.Resource.Name), refPath)

	resource, _, err := utils2.ResolveResourceReference(compvers, resourceRef, compvers.Repository())
	if err != nil {
		return fmt.Errorf("failed to fetch resource from component version: %w", err)
	}

	accSpec, err := resource.Access()
	if err != nil {
		return err
	}

	var (
		ref    string
		refErr error
	)

	for ref == "" && refErr == nil {
		switch x := accSpec.(type) {
		case *ociartifact.AccessSpec:
			ref = x.ImageReference
		case *ociblob.AccessSpec:
			ref = fmt.Sprintf("%s@%s", x.Reference, x.Digest)
		case *localblob.AccessSpec:
			if x.GlobalAccess == nil {
				refErr = errors.New("cannot determine image digest")
			} else {
				accSpec, refErr = octx.AccessSpecForSpec(x.GlobalAccess)
			}
		default:
			refErr = errors.New("cannot determine access spec type")
		}
	}

	if refErr != nil {
		return fmt.Errorf("failed to parse access reference: %w", refErr)
	}

	pRef, err := name.ParseReference(ref)
	if err != nil {
		return fmt.Errorf("failed to parse access reference: %w", err)
	}

	if l.Registry != "" {
		if err := localizations.Add("registry", l.File, l.Registry, pRef.Context().Registry.Name()); err != nil {
			return fmt.Errorf("failed to add registry: %w", err)
		}
	}

	if l.Repository != "" {
		if err := localizations.Add("repository", l.File, l.Repository, pRef.Context().RepositoryStr()); err != nil {
			return fmt.Errorf("failed to add repository: %w", err)
		}
	}

	if l.Image != "" {
		if err := localizations.Add("image", l.File, l.Image, pRef.Name()); err != nil {
			return fmt.Errorf("failed to add image ref name: %w", err)
		}
	}

	if l.Tag != "" {
		if err := localizations.Add("tag", l.File, l.Tag, pRef.Identifier()); err != nil {
			return fmt.Errorf("failed to add identifier: %w", err)
		}
	}

	return nil
}

func (m *MutationReconcileLooper) createSubstitutionRulesForConfigurationValues(
	data []byte,
	values *apiextensionsv1.JSON,
) (localize.Substitutions, error) {
	config := &configdata.ConfigData{}
	if err := ocmruntime.DefaultYAMLEncoding.Unmarshal(data, config); err != nil {
		return nil,
			fmt.Errorf("failed to unmarshal content: %w", err)
	}

	var rules localize.Substitutions
	for i, l := range config.Configuration.Rules {
		if err := rules.Add(fmt.Sprintf("subst-%d", i), l.File, l.Path, l.Value); err != nil {
			return nil, fmt.Errorf("failed to add rule: %w", err)
		}
	}

	defaults, err := json.Marshal(config.Configuration.Defaults)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal configuration defaults: %w", err) //nolint:staticcheck // it's fine
	}

	schema, err := json.Marshal(config.Configuration.Schema) //nolint:staticcheck // it's fine
	if err != nil {
		return nil, fmt.Errorf("failed to marshal configuration schema: %w", err)
	}

	configSubstitutions, err := m.generateSubstitutions(rules, defaults, values.Raw, schema)
	if err != nil {
		return nil, fmt.Errorf("configurator error: %w", err)
	}

	return configSubstitutions, nil
}

func (m *MutationReconcileLooper) generateSubstitutions(
	subst []localize.Substitution,
	defaults, values, schema []byte,
) (localize.Substitutions, error) {
	// configure defaults
	templ := make(map[string]any)
	if err := ocmruntime.DefaultYAMLEncoding.Unmarshal(defaults, &templ); err != nil {
		return nil, fmt.Errorf("cannot unmarshal template: %w", err)
	}

	// configure values overrides... must be a better way
	var valuesMap map[string]any
	if err := ocmruntime.DefaultYAMLEncoding.Unmarshal(values, &valuesMap); err != nil {
		return nil, fmt.Errorf("cannot unmarshal values: %w", err)
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
		return nil, fmt.Errorf("cannot marshal template: %w", err)
	}

	if len(schema) > 0 {
		if err := spiff.ValidateByScheme(values, schema); err != nil {
			return nil, fmt.Errorf("validation failed: %w", err)
		}
	}

	config, err := spiff.CascadeWith(spiff.TemplateData("adjustments", templateBytes), spiff.Mode(spiffing.MODE_PRIVATE))
	if err != nil {
		return nil, fmt.Errorf("error while doing cascade with: %w", err)
	}

	var result struct {
		Adjustments localize.Substitutions `json:"adjustments,omitempty"`
	}

	if err := ocmruntime.DefaultYAMLEncoding.Unmarshal(config, &result); err != nil {
		return nil, fmt.Errorf("error unmarshaling result: %w", err)
	}

	return result.Adjustments, nil
}

func (m *MutationReconcileLooper) compileMapping(ctx context.Context, cv *v1alpha1.ComponentVersion, mapping string) (json.RawMessage, error) {
	cueCtx := cuecontext.New()
	cd, err := component.GetComponentDescriptor(ctx, m.Client, nil, cv.Status.ComponentDescriptor)
	if err != nil {
		return nil, err
	}

	if cd == nil {
		return nil, fmt.Errorf("component descriptor not found with ref: %+v", cv.Status.ComponentDescriptor.ComponentDescriptorRef)
	}

	// first create the component descriptor struct
	root := cueCtx.CompileString("component:{}").FillPath(cue.ParsePath("component"), cueCtx.Encode(cd.Spec))

	// populate with refs
	root, err = m.populateReferences(ctx, root, cv.GetNamespace())
	if err != nil {
		return nil, err
	}

	// populate the mapping
	v := cueCtx.CompileString(mapping, cue.Scope(root))

	// resolve the output
	res, err := v.LookupPath(cue.ParsePath("out")).Bytes()
	if err != nil {
		return nil, err
	}

	// format the result
	var out json.RawMessage
	if err := json.Unmarshal(res, &out); err != nil {
		return nil, err
	}

	return out, nil
}

func (m *MutationReconcileLooper) populateReferences(ctx context.Context, src cue.Value, namespace string) (cue.Value, error) {
	root := src

	path := cue.ParsePath("component.references")

	refs := root.LookupPath(path)
	if !refs.Exists() {
		return src, nil
	}

	refList, err := refs.List()
	if err != nil {
		return src, err
	}

	for refList.Next() {
		val := refList.Value()
		index := refList.Selector()

		refData, err := val.Struct() //nolint:staticcheck // there exist no nicer option
		if err != nil {
			return src, err
		}

		refName, err := getStructFieldValue(refData, "componentName")
		if err != nil {
			return src, err
		}

		refVersion, err := getStructFieldValue(refData, "version")
		if err != nil {
			return src, err
		}

		refCDRef, err := component.ConstructUniqueName(refName, refVersion, ocmmetav1.Identity{})
		if err != nil {
			return src, err
		}

		ref := v1alpha1.Reference{
			Name:    refName,
			Version: refVersion,
			ComponentDescriptorRef: meta.NamespacedObjectReference{
				Namespace: namespace,
				Name:      refCDRef,
			},
		}

		cd, err := component.GetComponentDescriptor(ctx, m.Client, nil, ref)
		if err != nil {
			return src, err
		}

		val = val.FillPath(cue.ParsePath("component"), cd.Spec)

		val, err = m.populateReferences(ctx, val, namespace)
		if err != nil {
			return src, err
		}

		root = root.FillPath(cue.MakePath(cue.Str("component"), cue.Str("references"), index), val)
	}

	return root, nil
}

func getStructFieldValue(v *cue.Struct, field string) (string, error) {
	f, err := v.FieldByName(field, false)
	if err != nil {
		return "", err
	}

	return f.Value.String()
}

func (m *MutationReconcileLooper) getSource(ctx context.Context, ref meta.NamespacedObjectKindReference) (sourcev1.Source, error) {
	var obj client.Object
	switch ref.Kind {
	case sourcev1.GitRepositoryKind:
		obj = &sourcev1.GitRepository{}
	// case sourcev1.BucketKind:
	//     obj = &sourcev1.Bucket{}
	// case sourcev1.OCIRepositoryKind:
	//     obj = &sourcev1.OCIRepository{}
	default:
		return nil, fmt.Errorf("source `%s` kind '%s' not supported", ref.Name, ref.Kind)
	}

	key := types.NamespacedName{
		Name:      ref.Name,
		Namespace: ref.Namespace,
	}

	err := m.Client.Get(ctx, key, obj)
	if err != nil {
		return nil, fmt.Errorf("unable to get source '%s': %w", key, err)
	}

	source, ok := obj.(sourcev1.Source)
	if !ok {
		return nil, fmt.Errorf("object is not a source object: %+v", obj)
	}

	return source, nil
}

func (m *MutationReconcileLooper) getData(ctx context.Context, obj *v1alpha1.ObjectReference) ([]byte, error) {
	var (
		data []byte
		err  error
	)

	switch obj.Kind {
	case v1alpha1.ComponentVersionKind:
		if data, err = m.fetchDataFromComponentVersion(ctx, obj); err != nil {
			return nil,
				fmt.Errorf("failed to fetch resource data from resource ref: %w", err)
		}
	default:
		if data, err = m.fetchDataFromObjectReference(ctx, obj); err != nil {
			return nil,
				fmt.Errorf("failed to fetch resource data from snapshot: %w", err)
		}
	}

	return data, err
}

func (m *MutationReconcileLooper) getIdentity(ctx context.Context, obj *v1alpha1.ObjectReference) (ocmmetav1.Identity, error) {
	var (
		id  ocmmetav1.Identity
		err error
	)

	key := types.NamespacedName{
		Name:      obj.Name,
		Namespace: obj.Namespace,
	}

	switch obj.Kind {
	case v1alpha1.ComponentVersionKind:
		cv := &v1alpha1.ComponentVersion{}
		if err := m.Client.Get(ctx, key, cv); err != nil {
			return nil, err
		}

		id = ocmmetav1.Identity{
			v1alpha1.ComponentNameKey:    cv.Status.ComponentDescriptor.ComponentDescriptorRef.Name,
			v1alpha1.ComponentVersionKey: cv.Status.ComponentDescriptor.Version,
			v1alpha1.ResourceNameKey:     obj.ResourceRef.Name,
			v1alpha1.ResourceVersionKey:  obj.ResourceRef.Version,
		}
	default:
		// if kind is not ComponentVersion, then fetch resource using dynamic client
		// and get the snapshot name from the resource
		gvr := obj.GetGVR()
		src, err := m.DynamicClient.Resource(gvr).Namespace(obj.Namespace).Get(ctx, obj.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		snapshotName, ok, err := unstructured.NestedString(src.Object, "status", "snapshotName")
		if err != nil {
			return nil, fmt.Errorf("failed get the get snapshot: %w", err)
		}
		if !ok {
			return nil, errors.New("snapshot name not found in status")
		}

		snapshot := &v1alpha1.Snapshot{}
		if err := m.Client.Get(ctx, types.NamespacedName{Namespace: obj.Namespace, Name: snapshotName}, snapshot); err != nil {
			return nil, err
		}

		id = snapshot.Spec.Identity
	}

	return id, err
}

func (m *MutationReconcileLooper) getComponentVersion(ctx context.Context, obj *v1alpha1.ObjectReference) (*v1alpha1.ComponentVersion, error) {
	if obj.Kind != v1alpha1.ComponentVersionKind {
		return nil, errors.New("cannot retrieve component version for snapshot")
	}

	key := types.NamespacedName{
		Name:      obj.Name,
		Namespace: obj.Namespace,
	}
	cv := &v1alpha1.ComponentVersion{}
	if err := m.Client.Get(ctx, key, cv); err != nil {
		return nil, err
	}

	return cv, nil
}

// getValues returns values that can be used for the configuration
// currently it only possible to use inline values OR values from an external source.
func (m *MutationReconcileLooper) getValues(ctx context.Context, obj *v1alpha1.MutationSpec, namespace string) (*apiextensionsv1.JSON, error) {
	if obj.Values != nil {
		return obj.Values, nil
	}

	var data map[string]any
	if obj.ValuesFrom.FluxSource != nil {
		content, err := m.fromFluxSource(ctx, obj)
		if err != nil {
			return nil, fmt.Errorf("failed to get values from flux source: %w", err)
		}
		data = content
	} else if obj.ValuesFrom.ConfigMapSource != nil {
		content, err := m.fromConfigMapSource(ctx, obj, namespace)
		if err != nil {
			return nil, fmt.Errorf("failed to get values from configmap source: %w", err)
		}
		data = content
	}

	if data != nil {
		jsonData, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal values: %w", err)
		}

		return &apiextensionsv1.JSON{
			Raw: jsonData,
		}, nil
	}

	return nil, errors.New("no values found")
}

func (m *MutationReconcileLooper) fromConfigMapSource(ctx context.Context, obj *v1alpha1.MutationSpec, namespace string) (map[string]any, error) {
	data := make(map[string]any)
	cm := &corev1.ConfigMap{}
	key := types.NamespacedName{
		Name:      obj.ValuesFrom.ConfigMapSource.SourceRef.Name,
		Namespace: namespace,
	}
	if err := m.Client.Get(ctx, key, cm); err != nil {
		return nil, fmt.Errorf("failed to get configmap: %w", err)
	}

	content, found := cm.Data[obj.ValuesFrom.ConfigMapSource.Key]
	if !found {
		return nil, fmt.Errorf("key %s not found in configmap %s", obj.ValuesFrom.ConfigMapSource.Key, obj.ValuesFrom.ConfigMapSource.SourceRef.Name)
	}

	err := yaml.Unmarshal([]byte(content), &data)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal values: %w", err)
	}

	if obj.ValuesFrom.ConfigMapSource.SubPath != "" {
		data, found = extractSubpath(data, obj.ValuesFrom.ConfigMapSource.SubPath)
		if !found {
			return nil, errors.New("subPath not found")
		}
	}

	return data, nil
}

func (m *MutationReconcileLooper) fromFluxSource(ctx context.Context, obj *v1alpha1.MutationSpec) (map[string]any, error) {
	data := make(map[string]any)
	source, err := m.getSource(ctx, obj.ValuesFrom.FluxSource.SourceRef)
	if err != nil {
		return nil, fmt.Errorf("could not get values from source: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "mutation-controller-")
	if err != nil {
		return nil, fmt.Errorf("could not create temporary directory: %w", err)
	}

	tarSize := tar.UnlimitedUntarSize
	const retries = 10
	fetcher := fetch.NewArchiveFetcher(retries, tarSize, tarSize, "")
	artifact := source.GetArtifact()
	if artifact == nil {
		return nil, fmt.Errorf("could not get artifact from source: %s", obj.ValuesFrom.FluxSource.SourceRef.Name)
	}
	err = fetcher.Fetch(artifact.URL, source.GetArtifact().Digest, tmpDir)
	if err != nil {
		return nil, fmt.Errorf("could not fetch values artifact from source: %w", err)
	}

	path, err := securejoin.SecureJoin(tmpDir, obj.ValuesFrom.FluxSource.Path)
	if err != nil {
		return nil, fmt.Errorf("could not construct values file path: %w", err)
	}

	dataFile, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read values file: %w", err)
	}

	if err := yaml.Unmarshal(dataFile, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal values: %w", err)
	}

	var found bool
	if obj.ValuesFrom.FluxSource.SubPath != "" {
		data, found = extractSubpath(data, obj.ValuesFrom.FluxSource.SubPath)
		if !found {
			return nil, errors.New("subPath not found")
		}
	}

	return data, nil
}

func (m *MutationReconcileLooper) mutate(
	ctx context.Context,
	mutationSpec *v1alpha1.MutationSpec,
	sourceData, configData []byte,
	namespace string,
) (string, error) {
	// if values are not nil then this is configuration
	if mutationSpec.Values != nil || mutationSpec.ValuesFrom != nil {
		sourceDir, err := m.configure(ctx, sourceData, configData, mutationSpec, namespace)
		if err != nil {
			return "", fmt.Errorf("failed to configure resource: %w", err)
		}

		return sourceDir, nil
	}

	// if values are nil then this is localization
	return m.localize(ctx, mutationSpec, sourceData, configData)
}

// Recursive function to extract the subpath from the data map.
func extractSubpath(data map[string]any, subpath string) (map[string]any, bool) {
	keys := splitSubpath(subpath)
	curr := data

	for i, key := range keys {
		value, ok := curr[key]
		if !ok {
			return nil, false
		}

		if i == len(keys)-1 {
			switch t := value.(type) {
			case map[string]any:
				return t, true
			case map[any]any:
				return convertMap(t), true
			}

			return nil, false
		}

		switch t := value.(type) {
		case map[any]any:
			curr = convertMap(t)
		case map[string]any:
			curr = t
		default:
			return nil, false
		}
	}

	return nil, false
}

// Helper function to split the subpath into keys.
func splitSubpath(subpath string) []string {
	return strings.Split(subpath, ".")
}

func convertMap(data map[any]any) map[string]any {
	result := make(map[string]any)

	for k, v := range data {
		key, ok := k.(string)
		if !ok {
			// Handle the case where the key is not a string
			continue
		}

		switch value := v.(type) {
		case map[any]any:
			// Recursively convert nested maps
			result[key] = convertMap(value)
		default:
			result[key] = value
		}
	}

	return result
}
