package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/features"

	"github.com/open-component-model/ocm-e2e-framework/shared"
	"github.com/open-component-model/ocm-e2e-framework/shared/steps/setup"
)

var (
	BasePath                     = "testdata/common"
	componentNamePrefix          = "component.name."
	podinfoComponentName         = "/podinfo"
	podinfoBackendComponentName  = "/podinfo/backend"
	podinfoFrontendComponentName = "/podinfo/frontend"
	redisComponentName           = "/redis"
)

func createTestComponentVersion(t *testing.T, componentNameIdentifier string) *features.FeatureBuilder {
	t.Helper()
	return features.New("Add components to component-version").
		Setup(setup.AddComponentVersions(podinfo(t, componentNameIdentifier))).
		Setup(setup.AddComponentVersions(podinfoBackend(t, componentNameIdentifier))).
		Setup(setup.AddComponentVersions(podinfoFrontend(t, componentNameIdentifier))).
		Setup(setup.AddComponentVersions(podinfoRedis(t, componentNameIdentifier)))
}

func podinfo(t *testing.T, componentNameIdentifier string) setup.Component {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(BasePath, "product_description.yaml"))
	if err != nil {
		t.Fatal("failed to read setup file: %w", err)
	}

	return setup.Component{
		Component: shared.Component{
			Name:    componentNamePrefix + componentNameIdentifier + podinfoComponentName,
			Version: "1.0.0",
		},
		Repository: "podinfo",
		ComponentVersionModifications: []shared.ComponentModification{
			shared.BlobResource(shared.Resource{
				Name: "product-description",
				Data: string(content),
				Type: "productdescription.mpas.ocm.software",
			}),
			shared.ComponentVersionRef(shared.ComponentRef{
				Name:          "backend",
				Version:       "1.0.0",
				ComponentName: componentNamePrefix + componentNameIdentifier + podinfoBackendComponentName,
			}),
			shared.ComponentVersionRef(shared.ComponentRef{
				Name:          "frontend",
				Version:       "1.0.0",
				ComponentName: componentNamePrefix + componentNameIdentifier + podinfoFrontendComponentName,
			}),
			shared.ComponentVersionRef(shared.ComponentRef{
				Name:          "redis",
				Version:       "1.0.0",
				ComponentName: componentNamePrefix + componentNameIdentifier + redisComponentName,
			}),
		},
	}
}

func podinfoBackend(t *testing.T, componentNameIdentifier string) setup.Component {
	t.Helper()

	configContent, err := os.ReadFile(filepath.Join(BasePath, "podinfo", "backend", "config.yaml"))
	if err != nil {
		t.Fatal("failed to read config file: %w", err)
	}

	readmeContent, err := os.ReadFile(filepath.Join(BasePath, "podinfo", "backend", "README.md"))
	if err != nil {
		t.Fatal("failed to read readme file: %w", err)
	}

	manifestContent, err := os.ReadFile(filepath.Join(BasePath, "podinfo", "backend", "manifests.tar"))
	if err != nil {
		t.Fatal("failed to read manifest file: %w", err)
	}

	validationContent, err := os.ReadFile(filepath.Join(BasePath, "podinfo", "backend", "validation.rego"))
	if err != nil {
		t.Fatal("failed to read validation file: %w", err)
	}

	return setup.Component{
		Component: shared.Component{
			Name:    componentNamePrefix + componentNameIdentifier + podinfoBackendComponentName,
			Version: "1.0.0",
		},
		Repository: "backend",
		ComponentVersionModifications: []shared.ComponentModification{
			shared.BlobResource(shared.Resource{
				Name: "config",
				Data: string(configContent),
				Type: "configdata.ocm.software",
			}),
			shared.BlobResource(shared.Resource{
				Name: "instructions",
				Data: string(readmeContent),
				Type: "PlainText",
			}),
			shared.ImageRefResource("ghcr.io/stefanprodan/podinfo:6.2.0", shared.Resource{
				Name:    "image",
				Version: "6.2.0",
				Type:    "ociImage",
			}),
			shared.BlobResource(shared.Resource{
				Name: "manifests",
				Data: string(manifestContent),
				Type: "kustomize.ocm.fluxcd.io",
			}),
			shared.BlobResource(shared.Resource{
				Name: "validation",
				Data: string(validationContent),
				Type: "validator.mpas.ocm.software",
			}),
		},
	}
}

func podinfoFrontend(t *testing.T, componentNameIdentifier string) setup.Component {
	t.Helper()

	configContent, err := os.ReadFile(filepath.Join(BasePath, "podinfo", "frontend", "config.yaml"))
	if err != nil {
		t.Fatal("failed to read config file: %w", err)
	}

	readmeContent, err := os.ReadFile(filepath.Join(BasePath, "podinfo", "frontend", "README.md"))
	if err != nil {
		t.Fatal("failed to read readme file: %w", err)
	}

	manifestContent, err := os.ReadFile(filepath.Join(BasePath, "podinfo", "frontend", "manifests.tar"))
	if err != nil {
		t.Fatal("failed to read manifest file: %w", err)
	}

	validationContent, err := os.ReadFile(filepath.Join(BasePath, "podinfo", "frontend", "validation.rego"))
	if err != nil {
		t.Fatal("failed to read validation file: %w", err)
	}

	return setup.Component{
		Component: shared.Component{
			Name:    componentNamePrefix + componentNameIdentifier + podinfoFrontendComponentName,
			Version: "1.0.0",
		},
		Repository: "frontend",
		ComponentVersionModifications: []shared.ComponentModification{
			shared.BlobResource(shared.Resource{
				Name: "config",
				Data: string(configContent),
				Type: "configdata.ocm.software",
			}),
			shared.BlobResource(shared.Resource{
				Name: "instructions",
				Data: string(readmeContent),
				Type: "PlainText",
			}),
			shared.ImageRefResource("ghcr.io/stefanprodan/podinfo:6.2.0", shared.Resource{
				Name:    "image",
				Version: "6.2.0",
				Type:    "ociImage",
			}),
			shared.BlobResource(shared.Resource{
				Name: "manifests",
				Data: string(manifestContent),
				Type: "kustomize.ocm.fluxcd.io",
			}),
			shared.BlobResource(shared.Resource{
				Name: "validation",
				Data: string(validationContent),
				Type: "validator.mpas.ocm.software",
			}),
		},
	}
}

func podinfoRedis(t *testing.T, componentNameIdentifier string) setup.Component {
	t.Helper()

	configContent, err := os.ReadFile(filepath.Join(BasePath, "podinfo", "redis", "config.yaml"))
	if err != nil {
		t.Fatal("failed to read config file: %w", err)
	}

	readmeContent, err := os.ReadFile(filepath.Join(BasePath, "podinfo", "redis", "README.md"))
	if err != nil {
		t.Fatal("failed to read readme file: %w", err)
	}

	manifestContent, err := os.ReadFile(filepath.Join(BasePath, "podinfo", "redis", "manifests.tar"))
	if err != nil {
		t.Fatal("failed to read manifest file: %w", err)
	}

	validationContent, err := os.ReadFile(filepath.Join(BasePath, "podinfo", "redis", "validation.rego"))
	if err != nil {
		t.Fatal("failed to read validation file: %w", err)
	}

	return setup.Component{
		Component: shared.Component{
			Name:    componentNamePrefix + componentNameIdentifier + redisComponentName,
			Version: "1.0.0",
		},
		Repository: "redis",
		ComponentVersionModifications: []shared.ComponentModification{
			shared.BlobResource(shared.Resource{
				Name: "config",
				Data: string(configContent),
				Type: "configdata.ocm.software",
			}),
			shared.BlobResource(shared.Resource{
				Name: "instructions",
				Data: string(readmeContent),
				Type: "PlainText",
			}),
			shared.ImageRefResource("redis:6.0.1", shared.Resource{
				Name:    "image",
				Version: "6.2.0",
				Type:    "ociImage",
			}),
			shared.BlobResource(shared.Resource{
				Name: "manifests",
				Data: string(manifestContent),
				Type: "kustomize.ocm.fluxcd.io",
			}),
			shared.BlobResource(shared.Resource{
				Name: "validation",
				Data: string(validationContent),
				Type: "validator.mpas.ocm.software",
			}),
		},
	}
}
