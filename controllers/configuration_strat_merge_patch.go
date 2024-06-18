// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"

	securejoin "github.com/cyphar/filepath-securejoin"
	generator "github.com/fluxcd/pkg/kustomize"
	"github.com/fluxcd/pkg/tar"
	"gopkg.in/yaml.v2"
	kustypes "sigs.k8s.io/kustomize/api/types"
)

// the following is influenced by https://github.com/fluxcd/kustomize-controller
func (m *MutationReconcileLooper) strategicMergePatch(
	resource []byte,
	rootDir, workDir, sourcePath, targetPath string,
) (string, error) {
	gzipSnapshot := &bytes.Buffer{}
	gz := gzip.NewWriter(gzipSnapshot)
	if _, err := gz.Write(resource); err != nil {
		return "", err
	}

	if err := gz.Close(); err != nil {
		return "", err
	}

	if err := tar.Untar(gzipSnapshot, workDir); err != nil {
		return "", err
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
		return "", err
	}

	err = os.WriteFile(filepath.Join(workDir, "kustomization.yaml"), manifest, os.ModePerm)
	if err != nil {
		return "", err
	}

	result, err := generator.SecureBuild(rootDir, workDir, false)
	if err != nil {
		return "", err
	}

	contents, err := result.AsYaml()
	if err != nil {
		return "", err
	}

	outputPath, err := securejoin.SecureJoin(workDir, targetPath)
	if err != nil {
		return "", err
	}

	if err := os.Remove(outputPath); err != nil {
		return "", err
	}

	patched, err := os.Create(outputPath)
	if err != nil {
		return "", err
	}

	if _, err := patched.Write(contents); err != nil {
		return "", err
	}

	if err := patched.Close(); err != nil {
		return "", err
	}

	return filepath.Dir(outputPath), nil
}
