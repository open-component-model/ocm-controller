//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"testing"

	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1beta2"
	sourcev1 "github.com/fluxcd/source-controller/api/v1beta2"
	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-e2e-framework/shared"
	"sigs.k8s.io/e2e-framework/pkg/features"

	"github.com/open-component-model/ocm-e2e-framework/shared/steps/setup"
)

func TestSameResource(t *testing.T) {
	management := features.New("Configure Management Repository").
		Setup(setup.AddScheme(v1alpha1.AddToScheme)).
		Setup(setup.AddScheme(sourcev1.AddToScheme)).
		Setup(setup.AddScheme(kustomizev1.AddToScheme)).
		Setup(setup.AddGitRepository(testRepoSameName)).
		Setup(setup.AddFluxSyncForRepo(testRepoSameName, destinationPrefix, ocmNamespace))

	configContent, err := os.ReadFile(filepath.Join("testdata", "testOCMControllerMultipleSameResources", "config.yaml"))
	if err != nil {
		t.Fatal("failed to read config file: %w", err)
	}

	manifestContent, err := os.ReadFile(filepath.Join("testdata", "testOCMControllerMultipleSameResources", "manifests.tar"))
	if err != nil {
		t.Fatal("failed to read config file: %w", err)
	}

	component := setup.Component{
		Component: shared.Component{
			Name:    "component.name.resources/same",
			Version: "v1.0.0",
		},
		ComponentVersionModifications: []shared.ComponentModification{
			shared.BlobResource(shared.Resource{
				Name:    "same",
				Data:    string(configContent),
				Type:    "configdata.ocm.software",
				Version: "1.0.0",
				ExtraIdentity: map[string]string{
					"type": "config",
				},
			}),
			shared.BlobResource(shared.Resource{
				Name:    "same",
				Data:    string(manifestContent),
				Type:    "kustomize.ocm.fluxcd.io",
				Version: "1.0.0",
				ExtraIdentity: map[string]string{
					"type": "manifest",
				},
			}),
		},
	}

	testName := "testOCMControllerMultipleSameResources"

	setupComponentFeature := features.New("Setup Component").Setup(setup.AddComponentVersions(component))
	componentVersionFeature := features.New("Create Manifests").
		Setup(setup.AddFilesToGitRepository(setup.File{
			RepoName:       testRepoSameName,
			SourceFilepath: filepath.Join(testName, "component_version.yaml"),
			DestFilepath:   destinationPrefix + testName + "component_version.yaml",
		}, setup.File{
			RepoName:       testRepoSameName,
			SourceFilepath: filepath.Join(testName, "resource1.yaml"),
			DestFilepath:   destinationPrefix + testName + "resource1.yaml",
		}, setup.File{
			RepoName:       testRepoSameName,
			SourceFilepath: filepath.Join(testName, "resource2.yaml"),
			DestFilepath:   destinationPrefix + testName + "resource2.yaml",
		})).Assess("check that component version component_version.yaml is ready and valid", checkIsComponentVersionReady("same-resource-component", ocmNamespace))

	resourceAssertFeatures := features.New("Validate Resources").Setup(checkIsResourceReady("resource-1")).Setup(checkIsResourceReady("resource-2"))

	testEnv.Test(t,
		setupComponentFeature.Feature(),
		management.Feature(),
		componentVersionFeature.Feature(),
		resourceAssertFeatures.Feature(),
	)
}
