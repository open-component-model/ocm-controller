package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/mandelsoft/spiff/spiffing"
	"github.com/mandelsoft/vfs/pkg/osfs"
	"github.com/mandelsoft/vfs/pkg/vfs"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/utils/localize"
	ocmruntime "github.com/open-component-model/ocm/pkg/runtime"
	"github.com/open-component-model/ocm/pkg/spiff"
	"github.com/open-component-model/ocm/pkg/utils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/pkg/cache"
	"github.com/open-component-model/ocm-controller/pkg/component"
	"github.com/open-component-model/ocm-controller/pkg/configdata"
	"github.com/open-component-model/ocm-controller/pkg/ocm"
)

type MutationReconcileLooper struct {
	Scheme    *runtime.Scheme
	OCMClient ocm.FetchVerifier
	Client    client.Client
	Cache     cache.Cache
}

func (m *MutationReconcileLooper) ReconcileMutationObject(ctx context.Context, spec v1alpha1.MutationSpec, obj client.Object) (string, error) {
	log := log.FromContext(ctx)
	var (
		resourceData []byte
		err          error
	)

	cv := types.NamespacedName{
		Name:      spec.ComponentVersionRef.Name,
		Namespace: spec.ComponentVersionRef.Namespace,
	}

	componentVersion := &v1alpha1.ComponentVersion{}
	if err := m.Client.Get(ctx, cv, componentVersion); err != nil {
		if apierrors.IsNotFound(err) {
			return "", nil
		}

		return "",
			fmt.Errorf("failed to get component object: %w", err)
	}

	if spec.Source.SourceRef == nil && spec.Source.ResourceRef == nil {
		return "",
			fmt.Errorf("either sourceRef or resourceRef should be defined, but both are empty")
	}

	if spec.Source.SourceRef != nil {
		if resourceData, err = m.fetchResourceDataFromSnapshot(ctx, &spec); err != nil {
			return "",
				fmt.Errorf("failed to fetch resource data from snapshot: %w", err)
		}
	} else if spec.Source.ResourceRef != nil {
		if resourceData, err = m.fetchResourceDataFromResource(ctx, &spec, componentVersion); err != nil {
			return "",
				fmt.Errorf("failed to fetch resource data from resource ref: %w", err)
		}
	}

	if len(resourceData) == 0 {
		return "", fmt.Errorf("resource data cannot be empty")
	}

	// get config resource
	config := &configdata.ConfigData{}
	// TODO: allow for snapshots to be sources here. The chain could be working on an already modified source.
	resourceRef := spec.ConfigRef.Resource.ResourceRef
	if resourceRef == nil {
		return "",
			fmt.Errorf("resource ref is empty for config ref")
	}
	reader, _, err := m.OCMClient.GetResource(ctx, componentVersion, *resourceRef)
	if err != nil {
		return "",
			fmt.Errorf("failed to get resource: %w", err)
	}
	defer reader.Close()

	content, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read blob: %w", err)
	}

	if err := ocmruntime.DefaultYAMLEncoding.Unmarshal(content, config); err != nil {
		return "",
			fmt.Errorf("failed to unmarshal content: %w", err)
	}

	log.Info("preparing localization substitutions")

	componentDescriptor, err := component.GetComponentDescriptor(ctx, m.Client, spec.ConfigRef.Resource.ResourceRef.ReferencePath, componentVersion.Status.ComponentDescriptor)
	if err != nil {
		return "", fmt.Errorf("failed to get component descriptor from version")
	}
	if componentDescriptor == nil {
		return "", fmt.Errorf("couldn't find component descriptor for reference '%s' or any root components", spec.ConfigRef.Resource.ResourceRef.ReferencePath)
	}

	var rules localize.Substitutions
	if spec.Values != nil {
		rules, err = m.createSubstitutionRulesForConfigurationValues(spec, *config)
		if err != nil {
			return "", fmt.Errorf("failed to create configuration values for config '%s': %w", config.Name, err)
		}
	} else {
		rules, err = m.createSubstitutionRulesForLocalization(*componentDescriptor, *config)
		if err != nil {
			return "", fmt.Errorf("failed to create localization values for config '%s': %w", config.Name, err)
		}
	}

	virtualFS, err := osfs.NewTempFileSystem()
	if err != nil {
		return "", fmt.Errorf("fs error: %w", err)
	}
	defer vfs.Cleanup(virtualFS)

	if err := utils.ExtractTarToFs(virtualFS, bytes.NewBuffer(resourceData)); err != nil {
		return "", fmt.Errorf("extract tar error: %w", err)
	}

	if err := localize.Substitute(rules, virtualFS); err != nil {
		return "", fmt.Errorf("localization substitution failed: %w", err)
	}

	fi, err := virtualFS.Stat("/")
	if err != nil {
		return "", fmt.Errorf("fs error: %w", err)
	}

	sourceDir := filepath.Join(os.TempDir(), fi.Name())

	artifactPath, err := os.CreateTemp("", "snapshot-artifact-*.tgz")
	if err != nil {
		return "", fmt.Errorf("fs error: %w", err)
	}
	defer os.Remove(artifactPath.Name())

	if err := BuildTar(artifactPath.Name(), sourceDir); err != nil {
		return "", fmt.Errorf("build tar error: %w", err)
	}

	version := resourceRef.Version
	if version == "" {
		version = "latest"
	}

	// Create a new Identity for the modified resource. We use the obj.ResourceVersion as TAG to
	// find it later on.
	identity := v1alpha1.Identity{
		v1alpha1.ComponentNameKey:    componentDescriptor.Name,
		v1alpha1.ComponentVersionKey: componentDescriptor.Spec.Version,
		v1alpha1.ResourceNameKey:     resourceRef.Name,
		v1alpha1.ResourceVersionKey:  version,
	}
	snapshotDigest, err := m.writeToCache(ctx, identity, artifactPath.Name(), obj.GetResourceVersion())
	if err != nil {
		return "", err
	}

	snapshotCR := &v1alpha1.Snapshot{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: obj.GetNamespace(),
			Name:      spec.SnapshotTemplate.Name,
		},
	}

	_, err = controllerutil.CreateOrUpdate(ctx, m.Client, snapshotCR, func() error {
		if snapshotCR.ObjectMeta.CreationTimestamp.IsZero() {
			if err := controllerutil.SetOwnerReference(obj, snapshotCR, m.Scheme); err != nil {
				return fmt.Errorf("failed to set owner reference on snapshot: %w", err)
			}
		}
		snapshotCR.Spec = v1alpha1.SnapshotSpec{
			Identity: identity,
		}
		return nil
	})

	if err != nil {
		return "",
			fmt.Errorf("failed to create or update component descriptor: %w", err)
	}

	newSnapshotCR := snapshotCR.DeepCopy()
	newSnapshotCR.Status.Digest = snapshotDigest
	newSnapshotCR.Status.Tag = obj.GetResourceVersion()
	if err := patchObject(ctx, m.Client, snapshotCR, newSnapshotCR); err != nil {
		return "",
			fmt.Errorf("failed to patch snapshot CR: %w", err)
	}

	return snapshotDigest, nil
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

func (m *MutationReconcileLooper) fetchResourceDataFromSnapshot(ctx context.Context, spec *v1alpha1.MutationSpec) ([]byte, error) {
	log := log.FromContext(ctx)
	srcSnapshot := &v1alpha1.Snapshot{}
	if err := m.Client.Get(ctx, spec.GetSourceSnapshotKey(), srcSnapshot); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("snapshot not found", "snapshot", spec.GetSourceSnapshotKey())
			return nil, err
		}
		return nil,
			fmt.Errorf("failed to get component object: %w", err)
	}

	if conditions.IsFalse(srcSnapshot, v1alpha1.SnapshotReady) {
		log.Info("snapshot not ready yet", "snapshot", srcSnapshot.Name)
		return nil, nil
	}
	log.Info("getting snapshot data from snapshot", "snapshot", srcSnapshot)
	srcSnapshotData, err := m.getSnapshotBytes(ctx, srcSnapshot)
	if err != nil {
		return nil, err
	}

	return srcSnapshotData, nil
}

func (m *MutationReconcileLooper) fetchResourceDataFromResource(ctx context.Context, spec *v1alpha1.MutationSpec, version *v1alpha1.ComponentVersion) ([]byte, error) {
	resource, _, err := m.OCMClient.GetResource(ctx, version, *spec.Source.ResourceRef)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch resource from resource ref: %w", err)
	}
	defer resource.Close()

	content, err := io.ReadAll(resource)
	if err != nil {
		return nil, fmt.Errorf("failed to read resource data: %w", err)
	}

	return content, nil
}

func (m *MutationReconcileLooper) getSnapshotBytes(ctx context.Context, snapshot *v1alpha1.Snapshot) ([]byte, error) {
	name, err := ocm.ConstructRepositoryName(snapshot.Spec.Identity)
	if err != nil {
		return nil, fmt.Errorf("failed to construct name: %w", err)
	}
	reader, err := m.Cache.FetchDataByDigest(ctx, name, snapshot.Status.Digest)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch data: %w", err)
	}

	return io.ReadAll(reader)
}

func (m *MutationReconcileLooper) createSubstitutionRulesForLocalization(componentDescriptor v1alpha1.ComponentDescriptor, config configdata.ConfigData) (localize.Substitutions, error) {
	var localizations localize.Substitutions
	for _, l := range config.Localization {
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

		if l.Repository != "" {

			if err := localizations.Add("repository", l.File, l.Repository, ref.Context().Name()); err != nil {
				return nil, fmt.Errorf("failed to add repository: %w", err)
			}
		}

		if l.Image != "" {
			if err := localizations.Add("image", l.File, l.Image, ref.Name()); err != nil {
				return nil, fmt.Errorf("failed to add image ref name: %w", err)
			}
		}

		if l.Tag != "" {
			if err := localizations.Add("image", l.File, l.Tag, ref.Identifier()); err != nil {
				return nil, fmt.Errorf("failed to add identifier: %w", err)
			}
		}
	}

	return localizations, nil
}
func (m *MutationReconcileLooper) createSubstitutionRulesForConfigurationValues(spec v1alpha1.MutationSpec, config configdata.ConfigData) (localize.Substitutions, error) {
	var rules localize.Substitutions
	for i, l := range config.Configuration.Rules {
		rules.Add(fmt.Sprintf("subst-%d", i), l.File, l.Path, l.Value)
	}

	defaults, err := json.Marshal(config.Configuration.Defaults)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal configuration defaults: %w", err)
	}

	values, err := json.Marshal(spec.Values)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal spec values: %w", err)
	}

	schema, err := json.Marshal(config.Configuration.Schema)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal configuration schema: %w", err)
	}

	configSubstitions, err := m.configurator(rules, defaults, values, schema)
	if err != nil {
		return nil, fmt.Errorf("configurator error: %w", err)
	}
	return configSubstitions, nil
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
