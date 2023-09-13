// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/open-component-model/ocm-e2e-framework/shared"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/e2e-framework/pkg/features"

	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1beta2"
	sourcev1 "github.com/fluxcd/source-controller/api/v1beta2"
	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-e2e-framework/shared/steps/setup"
)

func TestHelmChartResource(t *testing.T) {
	t.Log("running e2e helm chart test")

	charts, err := os.ReadFile(filepath.Join("testdata", testHelmChartBasedResource, "podinfo-6.3.5.tgz"))
	require.NoError(t, err)

	setupComponent := features.New("Add components to component-version").
		Setup(setup.AddComponentVersions(setup.Component{
			Component: shared.Component{
				Name:    "github.com/open-component-model/helm-test",
				Version: "1.0.0",
			},
			ComponentVersionModifications: []shared.ComponentModification{
				shared.BlobResource(shared.Resource{
					Name:    "charts",
					Data:    string(charts),
					Type:    "helmChart",
					Version: "6.3.5",
				}),
			},
		}))

	management := features.New("Configure Management Repository").
		Setup(setup.AddScheme(v1alpha1.AddToScheme)).
		Setup(setup.AddScheme(sourcev1.AddToScheme)).
		Setup(setup.AddScheme(kustomizev1.AddToScheme)).
		Setup(setup.AddGitRepository(testRepoHelmName)).
		Setup(setup.AddFluxSyncForRepo(testRepoHelmName, destinationPrefix, ocmNamespace))

	manifestFiles := features.New("Create Manifests and add them to the flux repository").
		Setup(setup.AddFilesToGitRepository(getHelmManifests(testHelmChartBasedResource, testRepoHelmName)...)).
		Assess("check that component version is ready and valid", checkIsComponentVersionReady("ocm-with-helm", ocmNamespace))

	validationDeploymentBackend := checkDeploymentReadiness("fluxdeployer-podinfo-pipeline-backend", "ghcr.io/stefanprodan/podinfo")

	testEnv.Test(t,
		setupComponent.Feature(),
		management.Feature(),
		manifestFiles.Feature(),
		validationDeploymentBackend.Feature(),
	)
}

func getHelmManifests(testName string, gitRepositoryName string) []setup.File {
	cvManifest := setup.File{
		RepoName:       gitRepositoryName,
		SourceFilepath: filepath.Join(testName, cvFile),
		DestFilepath:   destinationPrefix + testName + cvFile,
	}
	resourceManifest := setup.File{
		RepoName:       gitRepositoryName,
		SourceFilepath: filepath.Join(testName, resourceFile),
		DestFilepath:   destinationPrefix + testName + resourceFile,
	}
	deployerManifestBackend := setup.File{
		RepoName:       gitRepositoryName,
		SourceFilepath: filepath.Join(testName, deployerFile),
		DestFilepath:   destinationPrefix + testName + deployerFile,
	}

	return []setup.File{
		cvManifest,
		resourceManifest,
		deployerManifestBackend,
	}
}
