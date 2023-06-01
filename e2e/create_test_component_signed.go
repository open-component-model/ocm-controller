package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/features"

	"github.com/open-component-model/ocm-e2e-framework/shared"
	"github.com/open-component-model/ocm-e2e-framework/shared/steps/setup"
)

func createTestComponentVersionSigned(t *testing.T, privateKey []byte, privateKeyName string, publicKey []byte, publicKeyName string) *features.FeatureBuilder {
	t.Helper()

	return features.New("Add components to component-version").
		Setup(shared.CreateSecret(publicKeyName, publicKey)).
		Setup(setup.AddComponentVersions(podinfoBackendSigned(t, privateKey, privateKeyName))).
		Setup(setup.AddComponentVersions(podinfoFrontendSigned(t, privateKey, privateKeyName))).
		Setup(setup.AddComponentVersions(podinfoRedisSigned(t, privateKey, privateKeyName))).
		Setup(setup.AddComponentVersions(podinfoSigned(t, privateKey, privateKeyName)))
}

func podinfoSigned(t *testing.T, key []byte, privateKeyName string) setup.Component {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(pathComponent, "product_description.yaml"))
	if err != nil {
		t.Fatal("failed to read setup file: %w", err)
	}

	return setup.Component{
		Component: shared.Component{
			Name:    podinfoComponentName,
			Version: "1.0.0",
			Sign: &shared.Sign{
				Name: privateKeyName,
				Key:  key,
			},
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
				ComponentName: podinfoBackendComponentName,
			}),
			shared.ComponentVersionRef(shared.ComponentRef{
				Name:          "frontend",
				Version:       "1.0.0",
				ComponentName: podinfoFrontendComponentName,
			}),
			shared.ComponentVersionRef(shared.ComponentRef{
				Name:          "redis",
				Version:       "1.0.0",
				ComponentName: redisComponentName,
			}),
		},
	}
}

func podinfoBackendSigned(t *testing.T, key []byte, privateKeyName string) setup.Component {
	t.Helper()
	configContent, err := os.ReadFile(filepath.Join(pathComponent, "podinfo", "backend", "config.yaml"))
	if err != nil {
		t.Fatal("failed to read config file: %w", err)
	}

	readmeContent, err := os.ReadFile(filepath.Join(pathComponent, "podinfo", "backend", "README.md"))
	if err != nil {
		t.Fatal("failed to read readme file: %w", err)
	}

	manifestContent, err := os.ReadFile(filepath.Join(pathComponent, "podinfo", "backend", "manifests.tar"))
	if err != nil {
		t.Fatal("failed to read manifest file: %w", err)
	}

	validationContent, err := os.ReadFile(filepath.Join(pathComponent, "podinfo", "backend", "validation.rego"))
	if err != nil {
		t.Fatal("failed to read validation file: %w", err)
	}

	return setup.Component{
		Component: shared.Component{
			Name:    podinfoBackendComponentName,
			Version: "1.0.0",
			Sign: &shared.Sign{
				Name: privateKeyName,
				Key:  key,
			},
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

func podinfoFrontendSigned(t *testing.T, key []byte, privateKeyName string) setup.Component {
	t.Helper()

	configContent, err := os.ReadFile(filepath.Join(pathComponent, "podinfo", "frontend", "config.yaml"))
	if err != nil {
		t.Fatal("failed to read config file: %w", err)
	}

	readmeContent, err := os.ReadFile(filepath.Join(pathComponent, "podinfo", "frontend", "README.md"))
	if err != nil {
		t.Fatal("failed to read readme file: %w", err)
	}

	manifestContent, err := os.ReadFile(filepath.Join(pathComponent, "podinfo", "frontend", "manifests.tar"))
	if err != nil {
		t.Fatal("failed to read manifest file: %w", err)
	}

	validationContent, err := os.ReadFile(filepath.Join(pathComponent, "podinfo", "frontend", "validation.rego"))
	if err != nil {
		t.Fatal("failed to read validation file: %w", err)
	}

	return setup.Component{
		Component: shared.Component{
			Name:    podinfoFrontendComponentName,
			Version: "1.0.0",
			Sign: &shared.Sign{
				Name: privateKeyName,
				Key:  key,
			},
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

func podinfoRedisSigned(t *testing.T, key []byte, privateKeyName string) setup.Component {
	t.Helper()

	configContent, err := os.ReadFile(filepath.Join(pathComponent, "podinfo", "redis", "config.yaml"))
	if err != nil {
		t.Fatal("failed to read config file: %w", err)
	}

	readmeContent, err := os.ReadFile(filepath.Join(pathComponent, "podinfo", "redis", "README.md"))
	if err != nil {
		t.Fatal("failed to read readme file: %w", err)
	}

	manifestContent, err := os.ReadFile(filepath.Join(pathComponent, "podinfo", "redis", "manifests.tar"))
	if err != nil {
		t.Fatal("failed to read manifest file: %w", err)
	}

	validationContent, err := os.ReadFile(filepath.Join(pathComponent, "podinfo", "redis", "validation.rego"))
	if err != nil {
		t.Fatal("failed to read validation file: %w", err)
	}

	return setup.Component{
		Component: shared.Component{
			Name:    redisComponentName,
			Version: "1.0.0",
			Sign: &shared.Sign{
				Name: privateKeyName,
				Key:  key,
			},
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
