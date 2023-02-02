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
	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"gopkg.in/yaml.v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kustypes "sigs.k8s.io/kustomize/api/types"
)

// the following is influenced by https://github.com/fluxcd/kustomize-controller
func (m *MutationReconcileLooper) strategicMergePatch(ctx context.Context, spec v1alpha1.MutationSpec, resource []byte, tmpDir string) (string, v1alpha1.Identity, error) {
	source, err := m.getSource(ctx, spec.PatchStrategicMerge.Source.SourceRef)
	if err != nil {
		return "", nil, err
	}

	workDir, err := securejoin.SecureJoin(tmpDir, "work")
	if err != nil {
		return "", nil, err
	}

	dirPath, err := securejoin.SecureJoin(workDir, spec.PatchStrategicMerge.Source.Path)
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
	err = fetcher.Fetch(source.GetArtifact().URL, source.GetArtifact().Checksum, workDir)
	if err != nil {
		return "", nil, err
	}

	kus := kustypes.Kustomization{
		TypeMeta: kustypes.TypeMeta{
			APIVersion: kustypes.KustomizationVersion,
			Kind:       kustypes.KustomizationKind,
		},
		Resources: []string{
			filepath.Base(dirPath),
		},
		PatchesStrategicMerge: []kustypes.PatchStrategicMerge{
			kustypes.PatchStrategicMerge(spec.PatchStrategicMerge.Target.Path),
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

	targetPath, err := securejoin.SecureJoin(workDir, spec.PatchStrategicMerge.Target.Path)
	if err != nil {
		return "", nil, err
	}

	if err := os.Remove(targetPath); err != nil {
		return "", nil, err
	}

	patched, err := os.Create(targetPath)
	if err != nil {
		return "", nil, err
	}

	if _, err := patched.Write(contents); err != nil {
		return "", nil, err
	}

	patched.Close()

	sourceDir := filepath.Dir(targetPath)

	identity := v1alpha1.Identity{
		v1alpha1.SourceNameKey:             source.(client.Object).GetName(),
		v1alpha1.SourceNamespaceKey:        source.(client.Object).GetNamespace(),
		v1alpha1.SourceArtifactChecksumKey: source.GetArtifact().Checksum,
	}

	return sourceDir, identity, nil
}
