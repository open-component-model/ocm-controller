package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/features"

	"github.com/open-component-model/ocm-e2e-framework/shared"
	"github.com/open-component-model/ocm-e2e-framework/shared/steps/setup"
)

func createTestComponentVersion(t *testing.T) *features.FeatureBuilder {
	t.Helper()

	return features.New("Setup OCM component for testing").
		Setup(setup.AddComponentVersions(podinfo(t))).
		Setup(setup.AddComponentVersions(podinfoBackend(t))).
		Setup(setup.AddComponentVersions(podinfoFrontend(t))).
		Setup(setup.AddComponentVersions(podinfoRedis(t)))
}

func podinfo(t *testing.T) setup.Component {
	t.Helper()

	content, err := os.ReadFile(filepath.Join("testdata", "product_description.yaml"))
	if err != nil {
		t.Fatal("failed to read setup file: %w", err)
	}

	return setup.Component{
		Component: shared.Component{
			Name:    "mpas.ocm.software/podinfo",
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
				ComponentName: "mpas.ocm.software/podinfo/backend",
			}),
			shared.ComponentVersionRef(shared.ComponentRef{
				Name:          "frontend",
				Version:       "1.0.0",
				ComponentName: "mpas.ocm.software/podinfo/frontend",
			}),
			shared.ComponentVersionRef(shared.ComponentRef{
				Name:          "redis",
				Version:       "1.0.0",
				ComponentName: "mpas.ocm.software/podinfo/redis",
			}),
		},
	}
}

func podinfoBackend(t *testing.T) setup.Component {
	t.Helper()

	configContent, err := os.ReadFile(filepath.Join("testdata", "podinfo", "backend", "config.yaml"))
	if err != nil {
		t.Fatal("failed to read config file: %w", err)
	}

	readmeContent, err := os.ReadFile(filepath.Join("testdata", "podinfo", "backend", "README.md"))
	if err != nil {
		t.Fatal("failed to read readme file: %w", err)
	}

	manifestContent, err := os.ReadFile(filepath.Join("testdata", "podinfo", "backend", "manifests.tar"))
	if err != nil {
		t.Fatal("failed to read manifest file: %w", err)
	}

	validationContent, err := os.ReadFile(filepath.Join("testdata", "podinfo", "backend", "validation.rego"))
	if err != nil {
		t.Fatal("failed to read validation file: %w", err)
	}

	return setup.Component{
		Component: shared.Component{
			Name:    "mpas.ocm.software/podinfo/backend",
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

func podinfoFrontend(t *testing.T) setup.Component {
	t.Helper()

	configContent, err := os.ReadFile(filepath.Join("testdata", "podinfo", "frontend", "config.yaml"))
	if err != nil {
		t.Fatal("failed to read config file: %w", err)
	}

	readmeContent, err := os.ReadFile(filepath.Join("testdata", "podinfo", "frontend", "README.md"))
	if err != nil {
		t.Fatal("failed to read readme file: %w", err)
	}

	manifestContent, err := os.ReadFile(filepath.Join("testdata", "podinfo", "frontend", "manifests.tar"))
	if err != nil {
		t.Fatal("failed to read manifest file: %w", err)
	}

	validationContent, err := os.ReadFile(filepath.Join("testdata", "podinfo", "frontend", "validation.rego"))
	if err != nil {
		t.Fatal("failed to read validation file: %w", err)
	}

	return setup.Component{
		Component: shared.Component{
			Name:    "mpas.ocm.software/podinfo/frontend",
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

func podinfoRedis(t *testing.T) setup.Component {
	t.Helper()

	configContent, err := os.ReadFile(filepath.Join("testdata", "podinfo", "redis", "config.yaml"))
	if err != nil {
		t.Fatal("failed to read config file: %w", err)
	}

	readmeContent, err := os.ReadFile(filepath.Join("testdata", "podinfo", "redis", "README.md"))
	if err != nil {
		t.Fatal("failed to read readme file: %w", err)
	}

	manifestContent, err := os.ReadFile(filepath.Join("testdata", "podinfo", "redis", "manifests.tar"))
	if err != nil {
		t.Fatal("failed to read manifest file: %w", err)
	}

	validationContent, err := os.ReadFile(filepath.Join("testdata", "podinfo", "redis", "validation.rego"))
	if err != nil {
		t.Fatal("failed to read validation file: %w", err)
	}

	return setup.Component{
		Component: shared.Component{
			Name:    "mpas.ocm.software/redis",
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
