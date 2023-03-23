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

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"github.com/containers/image/v5/pkg/compression"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	sourcev1 "github.com/fluxcd/source-controller/api/v1beta2"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/mandelsoft/spiff/spiffing"
	"github.com/mandelsoft/vfs/pkg/osfs"
	"github.com/mandelsoft/vfs/pkg/vfs"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/utils/localize"
	ocmruntime "github.com/open-component-model/ocm/pkg/runtime"
	"github.com/open-component-model/ocm/pkg/spiff"
	"github.com/open-component-model/ocm/pkg/utils"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/pkg/cache"
	"github.com/open-component-model/ocm-controller/pkg/component"
	"github.com/open-component-model/ocm-controller/pkg/configdata"
	"github.com/open-component-model/ocm-controller/pkg/ocm"
)

// tarError defines an error that occurs when the resource is not a tar archive.
var tarError = errors.New("expected tarred directory content for configuration/localization resources, got plain text")

type MutationReconcileLooper struct {
	Scheme    *runtime.Scheme
	OCMClient ocm.FetchVerifier
	Client    client.Client
	Cache     cache.Cache
}

func (m *MutationReconcileLooper) ReconcileMutationObject(ctx context.Context, componentVersion *v1alpha1.ComponentVersion, spec v1alpha1.MutationSpec, obj client.Object) (string, error) {
	log := log.FromContext(ctx)
	var (
		resourceData []byte
		resourceType string
		err          error
	)

	if spec.Source.SourceRef == nil && spec.Source.ResourceRef == nil {
		return "",
			fmt.Errorf("either sourceRef or resourceRef should be defined, but both are empty")
	}

	if spec.Source.SourceRef != nil {
		if resourceData, resourceType, err = m.fetchResourceDataFromSnapshot(ctx, &spec); err != nil {
			return "",
				fmt.Errorf("failed to fetch resource data from snapshot: %w", err)
		}
	} else if spec.Source.ResourceRef != nil {
		if resourceData, resourceType, err = m.fetchResourceDataFromResource(ctx, &spec, componentVersion); err != nil {
			return "",
				fmt.Errorf("failed to fetch resource data from resource ref: %w", err)
		}
	}

	if len(resourceData) == 0 {
		return "", fmt.Errorf("resource data cannot be empty")
	}

	var (
		identity  v1alpha1.Identity
		sourceDir string
	)

	if spec.ConfigRef != nil {
		virtualFS, err := osfs.NewTempFileSystem()
		if err != nil {
			return "", fmt.Errorf("fs error: %w", err)
		}

		defer func() {
			if err := vfs.Cleanup(virtualFS); err != nil {
				log.Error(err, "failed to cleanup virtual filesystem")
			}
		}()

		if !isTar(resourceData) {
			return "", tarError
		}

		if err := utils.ExtractTarToFs(virtualFS, bytes.NewBuffer(resourceData)); err != nil {
			return "", fmt.Errorf("extract tar error: %w", err)
		}

		fi, err := virtualFS.Stat("/")
		if err != nil {
			return "", fmt.Errorf("fs error: %w", err)
		}

		sourceDir = filepath.Join(os.TempDir(), fi.Name())

		var rules localize.Substitutions
		rules, identity, err = m.generateSubstRules(ctx, componentVersion, spec)
		if err != nil {
			return "", err
		}

		if len(rules) == 0 {
			log.Info("no rules generated from the available config data; the generate snapshot will have no modifications")
		}

		if err := localize.Substitute(rules, virtualFS); err != nil {
			return "", fmt.Errorf("localization substitution failed: %w", err)
		}
	}

	if spec.PatchStrategicMerge != nil {
		tmpDir, err := os.MkdirTemp("", "kustomization-")
		if err != nil {
			err = fmt.Errorf("tmp dir error: %w", err)
			return "", err
		}
		defer os.RemoveAll(tmpDir)

		sourceDir, identity, err = m.strategicMergePatch(ctx, spec, resourceData, tmpDir)
		if err != nil {
			return "", err
		}
	}

	return m.writeSnapshot(ctx, spec.SnapshotTemplate, obj, sourceDir, identity, resourceType)
}

func (m *MutationReconcileLooper) writeToCache(ctx context.Context, identity v1alpha1.Identity, artifactPath string, version string) (string, error) {
	file, err := os.Open(artifactPath)
	if err != nil {
		return "", fmt.Errorf("failed to open created archive: %w", err)
	}
	defer file.Close()
	name, err := ocm.ConstructRepositoryName(identity)
	if err != nil {
		return "", fmt.Errorf("failed to construct name: %w", err)
	}
	digest, err := m.Cache.PushData(ctx, file, name, version)
	if err != nil {
		return "", fmt.Errorf("failed to push blob to local registry: %w", err)
	}

	return digest, nil
}

func (m *MutationReconcileLooper) fetchResourceDataFromSnapshot(ctx context.Context, spec *v1alpha1.MutationSpec) ([]byte, string, error) {
	log := log.FromContext(ctx)
	srcSnapshot := &v1alpha1.Snapshot{}
	if err := m.Client.Get(ctx, spec.GetSourceSnapshotKey(), srcSnapshot); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("snapshot doesn't exist yet", "snapshot", spec.GetSourceSnapshotKey())
			return nil, "", err
		}
		return nil, "", fmt.Errorf("failed to get component object: %w", err)
	}

	if conditions.IsFalse(srcSnapshot, meta.ReadyCondition) {
		log.Info("snapshot not ready yet", "snapshot", srcSnapshot.Name)
		return nil, "", nil
	}
	log.Info("getting snapshot data from snapshot", "snapshot", srcSnapshot)
	srcSnapshotData, err := m.getSnapshotBytes(ctx, srcSnapshot)
	if err != nil {
		return nil, "", err
	}

	return srcSnapshotData, srcSnapshot.GetContentType(), nil
}

func (m *MutationReconcileLooper) fetchResourceDataFromResource(ctx context.Context, spec *v1alpha1.MutationSpec, version *v1alpha1.ComponentVersion) ([]byte, string, error) {
	resource, _, err := m.OCMClient.GetResource(ctx, version, *spec.Source.ResourceRef)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch resource from resource ref: %w", err)
	}
	defer resource.Close()

	componentDescriptor, err := component.GetComponentDescriptor(ctx, m.Client, spec.Source.ResourceRef.ReferencePath, version.Status.ComponentDescriptor)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get component descriptor from version")
	}

	if componentDescriptor == nil {
		return nil, "", fmt.Errorf("component descriptor was empty")
	}

	resourceObj := componentDescriptor.GetResource(spec.Source.ResourceRef.Name)
	if resourceObj == nil {
		return nil, "", fmt.Errorf("failed to find resource with name '%s' in component descriptor '%s'", spec.Source.ResourceRef.Name, componentDescriptor.Name)
	}

	uncompressed, _, err := compression.AutoDecompress(resource)
	if err != nil {
		return nil, "", fmt.Errorf("failed to auto decompress: %w", err)
	}
	defer uncompressed.Close()

	// This will be problematic with a 6 Gig large object when it's trying to read it all.
	content, err := io.ReadAll(uncompressed)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read resource data: %w", err)
	}

	return content, resourceObj.Type, nil
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

func (m *MutationReconcileLooper) createSubstitutionRulesForLocalization(ctx context.Context, componentDescriptor v1alpha1.ComponentDescriptor, config configdata.ConfigData) (localize.Substitutions, error) {
	var localizations localize.Substitutions
	for _, l := range config.Localization {
		if l.Mapping != nil {
			if len(componentDescriptor.GetOwnerReferences()) != 1 {
				return nil, errors.New("component descriptor has no owner component version")
			}
			parentKey := types.NamespacedName{
				Name:      componentDescriptor.GetOwnerReferences()[0].Name,
				Namespace: componentDescriptor.GetNamespace(),
			}
			parent := &v1alpha1.ComponentVersion{}
			if err := m.Client.Get(ctx, parentKey, parent); err != nil {
				return nil, err
			}
			res, err := m.compileMapping(ctx, parent, l.Mapping.Transform)
			if err != nil {
				return nil, fmt.Errorf("failed to compile mapping: %w", err)
			}
			if err := localizations.Add("custom", l.File, l.Mapping.Path, res); err != nil {
				return nil, fmt.Errorf("failed to add identifier: %w", err)
			}
			continue
		}

		lr := componentDescriptor.GetResource(l.Resource.Name)
		if lr == nil {
			continue
		}

		access, err := GetImageReference(lr)
		if err != nil {
			return nil,
				fmt.Errorf("failed to get image access: %w", err)
		}

		ref, err := name.ParseReference(access)
		if err != nil {
			return nil,
				fmt.Errorf("failed to parse access reference: %w", err)
		}

		if l.Registry != "" {
			if err := localizations.Add("registry", l.File, l.Registry, ref.Context().Registry.Name()); err != nil {
				return nil, fmt.Errorf("failed to add registry: %w", err)
			}
		}

		if l.Repository != "" {
			if err := localizations.Add("repository", l.File, l.Repository, ref.Context().RepositoryStr()); err != nil {
				return nil, fmt.Errorf("failed to add repository: %w", err)
			}
		}

		if l.Image != "" {
			if err := localizations.Add("image", l.File, l.Image, ref.Name()); err != nil {
				return nil, fmt.Errorf("failed to add image ref name: %w", err)
			}
		}

		if l.Tag != "" {
			if err := localizations.Add("tag", l.File, l.Tag, ref.Identifier()); err != nil {
				return nil, fmt.Errorf("failed to add identifier: %w", err)
			}
		}
	}

	return localizations, nil
}
func (m *MutationReconcileLooper) createSubstitutionRulesForConfigurationValues(spec v1alpha1.MutationSpec, config configdata.ConfigData) (localize.Substitutions, error) {
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

	values, err := json.Marshal(spec.Values)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal spec values: %w", err)
	}

	schema, err := json.Marshal(config.Configuration.Schema) //nolint:staticcheck // it's fine
	if err != nil {
		return nil, fmt.Errorf("failed to marshal configuration schema: %w", err)
	}

	configSubstitutions, err := m.configurator(rules, defaults, values, schema)
	if err != nil {
		return nil, fmt.Errorf("configurator error: %w", err)
	}
	return configSubstitutions, nil
}

func (m *MutationReconcileLooper) configurator(subst []localize.Substitution, defaults, values, schema []byte) (localize.Substitutions, error) {
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

		refData, err := val.Struct()
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

		refCDRef, err := component.ConstructUniqueName(refName, refVersion, v1.Identity{})
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

func (m *MutationReconcileLooper) getSource(ctx context.Context, ref v1alpha1.PatchStrategicMergeSourceRef) (sourcev1.Source, error) {
	var obj client.Object
	switch ref.Kind {
	case sourcev1.GitRepositoryKind:
		obj = &sourcev1.GitRepository{}
	case sourcev1.BucketKind:
		obj = &sourcev1.Bucket{}
	case sourcev1.OCIRepositoryKind:
		obj = &sourcev1.OCIRepository{}
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

	return obj.(sourcev1.Source), nil
}
