// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

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
	basePath                     = "testdata/common"
	componentNamePrefix          = "component.name."
	podinfoComponentName         = "/podinfo"
	podinfoBackendComponentName  = "/podinfo/backend"
	podinfoFrontendComponentName = "/podinfo/frontend"
	redisComponentName           = "/redis"
)

func createTestComponentVersionUnsigned(t *testing.T, componentNameIdentifier string) *features.FeatureBuilder {
	t.Helper()
	return features.New("Add components to component-version").
		Setup(setup.AddComponentVersions(podinfoBackend(t, nil, "", componentNameIdentifier))).
		Setup(setup.AddComponentVersions(podinfoFrontend(t, nil, "", componentNameIdentifier))).
		Setup(setup.AddComponentVersions(podinfoRedis(t, nil, "", componentNameIdentifier))).
		Setup(setup.AddComponentVersions(podinfo(t, nil, "", componentNameIdentifier)))
}

func createTestComponentVersionSigned(t *testing.T, featureString string, privateKey []byte, keyName string, publicKey []byte, componentNameIdentifier string) *features.FeatureBuilder {
	t.Helper()
	return features.New(featureString).
		WithStep("create secret", 1, shared.CreateSecret(keyName, publicKey)).
		WithStep("", 2, setup.AddComponentVersions(basicSignedComponent(t, privateKey, keyName, componentNameIdentifier)))
}

func podinfo(t *testing.T, privateKey []byte, privateKeyName string, componentNameIdentifier string) setup.Component {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(basePath, "product_description.yaml"))
	if err != nil {
		t.Fatal("failed to read setup file: %w", err)
	}

	temp := setup.Component{
		Component:  getComponent(privateKeyName, privateKey, componentNameIdentifier, podinfoComponentName),
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
	return temp
}

func basicSignedComponent(t *testing.T, privateKey []byte, privateKeyName string, componentNameIdentifier string) setup.Component {
	t.Helper()
	temp := setup.Component{
		Component: shared.Component{
			Name:    componentNamePrefix + componentNameIdentifier + podinfoComponentName,
			Version: "1.0.0",
			Sign: &shared.Sign{
				Name: privateKeyName,
				Key:  privateKey,
			},
		},
		Repository: "podinfo",
		ComponentVersionModifications: []shared.ComponentModification{
			shared.BlobResource(shared.Resource{
				Name: "product-description",
				Data: "test-component",
				Type: "PlainText",
			}),
		},
	}
	return temp
}

func podinfoBackend(t *testing.T, privateKey []byte, privateKeyName string, componentNameIdentifier string) setup.Component {
	t.Helper()

	configContent, err := os.ReadFile(filepath.Join(basePath, "podinfo", "backend", "config.yaml"))
	if err != nil {
		t.Fatal("failed to read config file: %w", err)
	}

	readmeContent, err := os.ReadFile(filepath.Join(basePath, "podinfo", "backend", "README.md"))
	if err != nil {
		t.Fatal("failed to read readme file: %w", err)
	}

	manifestContent, err := os.ReadFile(filepath.Join(basePath, "podinfo", "backend", "manifests.tar"))
	if err != nil {
		t.Fatal("failed to read manifest file: %w", err)
	}

	validationContent, err := os.ReadFile(filepath.Join(basePath, "podinfo", "backend", "validation.rego"))
	if err != nil {
		t.Fatal("failed to read validation file: %w", err)
	}

	return setup.Component{
		Component:  getComponent(privateKeyName, privateKey, componentNameIdentifier, podinfoBackendComponentName),
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

func podinfoFrontend(t *testing.T, privateKey []byte, privateKeyName string, componentNameIdentifier string) setup.Component {
	t.Helper()

	configContent, err := os.ReadFile(filepath.Join(basePath, "podinfo", "frontend", "config.yaml"))
	if err != nil {
		t.Fatal("failed to read config file: %w", err)
	}

	readmeContent, err := os.ReadFile(filepath.Join(basePath, "podinfo", "frontend", "README.md"))
	if err != nil {
		t.Fatal("failed to read readme file: %w", err)
	}

	manifestContent, err := os.ReadFile(filepath.Join(basePath, "podinfo", "frontend", "manifests.tar"))
	if err != nil {
		t.Fatal("failed to read manifest file: %w", err)
	}

	validationContent, err := os.ReadFile(filepath.Join(basePath, "podinfo", "frontend", "validation.rego"))
	if err != nil {
		t.Fatal("failed to read validation file: %w", err)
	}

	return setup.Component{
		Component:  getComponent(privateKeyName, privateKey, componentNameIdentifier, podinfoFrontendComponentName),
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

func podinfoRedis(t *testing.T, privateKey []byte, privateKeyName string, componentNameIdentifier string) setup.Component {
	t.Helper()

	configContent, err := os.ReadFile(filepath.Join(basePath, "podinfo", "redis", "config.yaml"))
	if err != nil {
		t.Fatal("failed to read config file: %w", err)
	}

	readmeContent, err := os.ReadFile(filepath.Join(basePath, "podinfo", "redis", "README.md"))
	if err != nil {
		t.Fatal("failed to read readme file: %w", err)
	}

	manifestContent, err := os.ReadFile(filepath.Join(basePath, "podinfo", "redis", "manifests.tar"))
	if err != nil {
		t.Fatal("failed to read manifest file: %w", err)
	}

	validationContent, err := os.ReadFile(filepath.Join(basePath, "podinfo", "redis", "validation.rego"))
	if err != nil {
		t.Fatal("failed to read validation file: %w", err)
	}

	return setup.Component{
		Component:  getComponent(privateKeyName, privateKey, componentNameIdentifier, redisComponentName),
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
func getComponent(privateKeyName string, privateKey []byte, componentNameIdentifier string, componentName string) shared.Component {
	if len(privateKeyName) > 0 && privateKey != nil {
		return shared.Component{
			Name:    componentNamePrefix + componentNameIdentifier + componentName,
			Version: "1.0.0",
			Sign: &shared.Sign{
				Name: privateKeyName,
				Key:  privateKey,
			},
		}
	}
	return shared.Component{
		Name:    componentNamePrefix + componentNameIdentifier + componentName,
		Version: "1.0.0",
	}
}
