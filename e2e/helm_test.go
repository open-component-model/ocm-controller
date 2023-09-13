// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package e2e

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/open-component-model/ocm-controller/pkg/oci"
	"github.com/open-component-model/ocm-e2e-framework/shared"
	"github.com/open-component-model/ocm-e2e-framework/shared/steps/teardown"
	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/accessmethods/helm"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc"
	ocmmetav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/registry"
	"sigs.k8s.io/e2e-framework/pkg/features"

	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1beta2"
	sourcev1 "github.com/fluxcd/source-controller/api/v1beta2"
	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-e2e-framework/shared/steps/setup"
)

func TestHelmChartResource(t *testing.T) {
	t.Log("running e2e helm chart test")

	charts, err := os.Open(filepath.Join("testdata", testHelmChartBasedResource, "podinfo-6.3.5.tgz"))
	require.NoError(t, err)

	cache := oci.NewClient("127.0.0.1:5000", oci.WithInsecureSkipVerify(true))
	_, err = cache.PushData(context.Background(), charts, registry.ChartLayerMediaType, "podinfo", "6.3.5")
	require.NoError(t, err)

	// create transport archive
	//octx := ocm.New(datacontext.MODE_SHARED)
	//src, err := ctf.Open(octx, accessobj.ACC_CREATE, filepath.Join("testdata", testHelmChartBasedResource, "component"), os.ModePerm, accessio.FormatDirectory)
	//require.NoError(t, err)
	//
	//component := "github.com/open-component-model/helm-test"
	//version := "v1.0.0"
	//
	//defer src.Close()
	//cv, err := src.LookupComponentVersion(component, version)
	//require.NoError(t, err)
	//
	//meta := ocireg.NewComponentRepositoryMeta(".", ocireg.OCIRegistryURLPathMapping)
	//targetSpec := ocireg.NewRepositorySpec("127.0.0.1:5000", meta)
	//target, err := octx.RepositoryForSpec(targetSpec)
	//require.NoError(t, err)
	//defer target.Close()
	//
	//handler, err := spiff.New(standard.ResourcesByValue())
	//err = transfer.TransferVersion(nil, nil, cv, target, handler)
	//require.NoError(t, err)
	//printer := common.NewPrinter(os.Stdout)
	//closure := transfer.TransportClosure{}
	//transferopts := &standard.Options{}
	//transferhandler.From(octx.ConfigContext(), transferopts)
	//transferhandler.ApplyOptions(transferopts,
	//	standard.Recursive(true),
	//	standard.ResourcesByValue(true),
	//	standard.Overwrite(),
	//	standard.Resolver(target))
	//transferHandler, err := standard.New(transferopts)
	//if err != nil {
	//	return err
	//}
	//err = transfer.TransferVersion(printer, closure, ctf, target, transferHandler)
	//if err != nil {
	//	return fmt.Errorf("cannot transfer version %s for component %s: %w", vname, cname, err)
	//}
	setupComponent := features.New("Add components to component-version").
		Setup(setup.AddComponentVersions(setup.Component{
			Component: shared.Component{
				Name:    "github.com/open-component-model/helm-test-component",
				Version: "1.0.0",
			},

			ComponentVersionModifications: []shared.ComponentModification{
				helmResource("podinfo", "registry.ocm-system.svc.cluster.local:5000/podinfo", shared.Resource{
					Name:    "podinfo",
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

	dumpState := features.New("dump cluster state").Teardown(teardown.DumpClusterState("ocm-controller"))
	dumpRepo := features.New("dump repository content").Teardown(teardown.DumpRepositoryContent(shared.Owner, testRepoHelmName))

	testEnv.Test(t,
		setupComponent.Feature(),
		management.Feature(),
		manifestFiles.Feature(),
		validationDeploymentBackend.Feature(),
		dumpState.Feature(),
		dumpRepo.Feature(),
	)
}

// ImageRefResource creates an image reference type resource.
func helmResource(chart, url string, resource shared.Resource) shared.ComponentModification {
	return func(compvers ocm.ComponentVersionAccess) error {
		return compvers.SetResource(&compdesc.ResourceMeta{
			ElementMeta: compdesc.ElementMeta{
				Name:    resource.Name,
				Version: resource.Version,
			},
			Type:     resource.Type,
			Relation: ocmmetav1.ExternalRelation,
		}, helm.New(chart, url))
	}
}

func getHelmManifests(testName string, gitRepositoryName string) []setup.File {
	cvManifest := setup.File{
		RepoName:       gitRepositoryName,
		SourceFilepath: filepath.Join(testName, cvFile),
		DestFilepath:   destinationPrefix + testName + "/" + cvFile,
	}
	resourceManifest := setup.File{
		RepoName:       gitRepositoryName,
		SourceFilepath: filepath.Join(testName, resourceFile),
		DestFilepath:   destinationPrefix + testName + "/" + resourceFile,
	}
	deployerManifestBackend := setup.File{
		RepoName:       gitRepositoryName,
		SourceFilepath: filepath.Join(testName, deployerFile),
		DestFilepath:   destinationPrefix + testName + "/" + deployerFile,
	}

	return []setup.File{
		cvManifest,
		resourceManifest,
		deployerManifestBackend,
	}
}
