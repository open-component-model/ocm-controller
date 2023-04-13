// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/fluxcd/pkg/http/fetch"
	generator "github.com/fluxcd/pkg/kustomize"
	"github.com/fluxcd/pkg/tar"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	ocmmetav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	"gopkg.in/yaml.v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kustypes "sigs.k8s.io/kustomize/api/types"
)

// the following is influenced by https://github.com/fluxcd/kustomize-controller
func (m *MutationReconcileLooper) strategicMergePatch(ctx context.Context,
	source sourcev1.Source,
	resource []byte,
	tmpDir,
	sourcePath,
	targetPath string) (string, ocmmetav1.Identity, error) {
	workDir, err := securejoin.SecureJoin(tmpDir, "work")
	if err != nil {
		return "", nil, err
	}

	gzipSnapshot := &bytes.Buffer{}
	gz := gzip.NewWriter(gzipSnapshot)
	if _, err := gz.Write(resource); err != nil {
		return "", nil, err
	}

	if err := gz.Close(); err != nil {
		return "", nil, err
	}

	if err := tar.Untar(gzipSnapshot, workDir); err != nil {
		return "", nil, err
	}

	tarSize := tar.UnlimitedUntarSize
	fetcher := fetch.NewArchiveFetcher(10, tarSize, tarSize, "")
	err = fetcher.Fetch(source.GetArtifact().URL, source.GetArtifact().Digest, workDir)
	if err != nil {
		return "", nil, err
	}

	kus := kustypes.Kustomization{
		TypeMeta: kustypes.TypeMeta{
			APIVersion: kustypes.KustomizationVersion,
			Kind:       kustypes.KustomizationKind,
		},
		Resources: []string{
			sourcePath,
		},
		PatchesStrategicMerge: []kustypes.PatchStrategicMerge{
			kustypes.PatchStrategicMerge(targetPath),
		},
	}

	manifest, err := yaml.Marshal(kus)
	if err != nil {
		return "", nil, err
	}

	err = os.WriteFile(filepath.Join(workDir, "kustomization.yaml"), manifest, os.ModePerm)
	if err != nil {
		return "", nil, err
	}

	result, err := generator.SecureBuild(tmpDir, workDir, false)
	if err != nil {
		return "", nil, err
	}

	contents, err := result.AsYaml()
	if err != nil {
		return "", nil, err
	}

	outputPath, err := securejoin.SecureJoin(workDir, targetPath)
	if err != nil {
		return "", nil, err
	}

	if err := os.Remove(outputPath); err != nil {
		return "", nil, err
	}

	patched, err := os.Create(outputPath)
	if err != nil {
		return "", nil, err
	}

	if _, err := patched.Write(contents); err != nil {
		return "", nil, err
	}

	if err := patched.Close(); err != nil {
		return "", nil, err
	}

	identity := ocmmetav1.Identity{
		v1alpha1.SourceNameKey:             source.(client.Object).GetName(),
		v1alpha1.SourceNamespaceKey:        source.(client.Object).GetNamespace(),
		v1alpha1.SourceArtifactChecksumKey: source.GetArtifact().Digest,
	}

	return filepath.Dir(outputPath), identity, nil
}
