package controllers

import (
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"

	securejoin "github.com/cyphar/filepath-securejoin"
	generator "github.com/fluxcd/pkg/kustomize"
	"github.com/fluxcd/pkg/tar"
	kustypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/yaml"
)

const (
	FSOwnerReadWrite = 0o600
)

// the following is influenced by https://github.com/fluxcd/kustomize-controller
func (m *MutationReconcileLooper) strategicMergePatch(
	resource []byte,
	rootDir, workDir, sourcePath, targetPath string,
) (string, error) {
	// remove the source path
	defer os.Remove(sourcePath)
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

	kustomize := filepath.Join(workDir, "kustomization.yaml")
	err = os.WriteFile(kustomize, manifest, FSOwnerReadWrite)
	if err != nil {
		return "", err
	}
	// remove the kustomize file
	defer os.Remove(kustomize)

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

	return filepath.Dir(workDir), nil
}
