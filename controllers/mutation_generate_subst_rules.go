package controllers

import (
	"context"
	"fmt"
	"io"

	"github.com/containers/image/v5/pkg/compression"
	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/pkg/component"
	"github.com/open-component-model/ocm-controller/pkg/configdata"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/utils/localize"
	ocmruntime "github.com/open-component-model/ocm/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (m *MutationReconcileLooper) generateSubstRules(ctx context.Context, cv *v1alpha1.ComponentVersion, spec v1alpha1.MutationSpec) (localize.Substitutions, v1alpha1.Identity, error) {
	log := log.FromContext(ctx)

	config := &configdata.ConfigData{}
	resourceRef := spec.ConfigRef.Resource.ResourceRef
	if resourceRef == nil {
		return nil, nil, fmt.Errorf("resource ref is empty for config ref")
	}

	reader, _, err := m.OCMClient.GetResource(ctx, cv, *resourceRef)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get resource: %w", err)
	}
	defer reader.Close()

	// This content might be Tarred up by OCM.
	uncompressed, _, err := compression.AutoDecompress(reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to auto decompress: %w", err)
	}
	defer uncompressed.Close()

	content, err := io.ReadAll(uncompressed)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read blob: %w", err)
	}

	if err := ocmruntime.DefaultYAMLEncoding.Unmarshal(content, config); err != nil {
		return nil, nil,
			fmt.Errorf("failed to unmarshal content: %w", err)
	}

	log.Info("preparing localization substitutions")

	componentDescriptor, err := component.GetComponentDescriptor(ctx, m.Client, spec.ConfigRef.Resource.ResourceRef.ReferencePath, cv.Status.ComponentDescriptor)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get component descriptor from version")
	}

	if componentDescriptor == nil {
		return nil, nil, fmt.Errorf("couldn't find component descriptor for reference '%s' or any root components", spec.ConfigRef.Resource.ResourceRef.ReferencePath)
	}

	var rules localize.Substitutions
	if spec.Values != nil {
		rules, err = m.createSubstitutionRulesForConfigurationValues(spec, *config)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create configuration values for config '%s': %w", config.Name, err)
		}
	} else {
		rules, err = m.createSubstitutionRulesForLocalization(ctx, *componentDescriptor, *config)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create localization values for config '%s': %w", config.Name, err)
		}
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

	return rules, identity, err
}
