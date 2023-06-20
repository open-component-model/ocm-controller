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
	basePath                     = "testdata/"
	componentNamePrefix          = "component.name."
	podinfoComponentName         = "/podinfo"
	podinfoBackendComponentName  = "/podinfo/backend"
	podinfoFrontendComponentName = "/podinfo/frontend"
	redisComponentName           = "/redis"
	podinfoName                  = "podinfo"
	backend                      = "backend"
	frontend                     = "frontend"
	redis                        = "redis"
	errMsgReadFile               = "failed to read file: %w"
)

func createTestComponentVersionUnsigned(t *testing.T, componentNameIdentifier string, testPath string, version string) *features.FeatureBuilder {
	t.Helper()
	return features.New("Add components to component-version").
		Setup(setup.AddComponentVersions(podinfoBackend(t, nil, "", componentNameIdentifier, testPath, version))).
		Setup(setup.AddComponentVersions(podinfoFrontend(t, nil, "", componentNameIdentifier, testPath, version))).
		Setup(setup.AddComponentVersions(podinfoRedis(t, nil, "", componentNameIdentifier, testPath, version))).
		Setup(setup.AddComponentVersions(podinfo(t, nil, "", componentNameIdentifier, testPath, version)))
}

func createTestComponentVersionSigned(t *testing.T, featureString string, privateKey []byte, keyName string, publicKey []byte, componentNameIdentifier string, testPath string, version string) *features.FeatureBuilder {
	t.Helper()
	return features.New(featureString).
		WithStep("create secret", 1, shared.CreateSecret(keyName, publicKey)).
		WithStep("", 2, setup.AddComponentVersions(podinfoBackend(t, privateKey, keyName, componentNameIdentifier, testPath, version))).
		WithStep("", 2, setup.AddComponentVersions(podinfoFrontend(t, privateKey, keyName, componentNameIdentifier, testPath, version))).
		WithStep("", 2, setup.AddComponentVersions(podinfoRedis(t, privateKey, keyName, componentNameIdentifier, testPath, version))).
		WithStep("", 2, setup.AddComponentVersions(podinfo(t, privateKey, keyName, componentNameIdentifier, testPath, version)))
}

func podinfo(t *testing.T, privateKey []byte, privateKeyName string, componentNameIdentifier string, testPath string, version string) setup.Component {
	t.Helper()

	return setup.Component{
		Component:  getComponent(privateKeyName, privateKey, componentNameIdentifier, podinfoComponentName, version),
		Repository: podinfoName,
		ComponentVersionModifications: []shared.ComponentModification{
			shared.ComponentVersionRef(shared.ComponentRef{
				Name:          backend,
				Version:       version,
				ComponentName: componentNamePrefix + componentNameIdentifier + podinfoBackendComponentName,
			}),
			shared.ComponentVersionRef(shared.ComponentRef{
				Name:          frontend,
				Version:       version,
				ComponentName: componentNamePrefix + componentNameIdentifier + podinfoFrontendComponentName,
			}),
			shared.ComponentVersionRef(shared.ComponentRef{
				Name:          redis,
				Version:       version,
				ComponentName: componentNamePrefix + componentNameIdentifier + redisComponentName,
			}),
		},
	}
}

func podinfoBackend(t *testing.T, privateKey []byte, privateKeyName string, componentNameIdentifier string, testPath string, version string) setup.Component {
	t.Helper()

	configContent, err := os.ReadFile(filepath.Join(basePath, testPath, podinfoName, backend, "config.yaml"))
	if err != nil {
		t.Fatal(errMsgReadFile, err)
	}

	readmeContent, err := os.ReadFile(filepath.Join(basePath, testPath, podinfoName, backend, "README.md"))
	if err != nil {
		t.Fatal(errMsgReadFile, err)
	}

	manifestContent, err := os.ReadFile(filepath.Join(basePath, testPath, podinfoName, backend, "manifests.tar"))
	if err != nil {
		t.Fatal(errMsgReadFile, err)
	}

	validationContent, err := os.ReadFile(filepath.Join(basePath, testPath, podinfoName, backend, "validation.rego"))
	if err != nil {
		t.Fatal(errMsgReadFile, err)
	}

	return setup.Component{
		Component:  getComponent(privateKeyName, privateKey, componentNameIdentifier, podinfoBackendComponentName, version),
		Repository: backend,
		ComponentVersionModifications: []shared.ComponentModification{
			shared.BlobResource(shared.Resource{
				Name:    "config",
				Data:    string(configContent),
				Type:    "configdata.ocm.software",
				Version: version,
			}),
			shared.BlobResource(shared.Resource{
				Name:    "instructions",
				Data:    string(readmeContent),
				Type:    "PlainText",
				Version: version,
			}),
			shared.ImageRefResource("ghcr.io/stefanprodan/podinfo:6.2.0", shared.Resource{
				Name:    "image",
				Version: "6.2.0",
				Type:    "ociImage",
			}),
			shared.BlobResource(shared.Resource{
				Name:    "manifests",
				Data:    string(manifestContent),
				Type:    "kustomize.ocm.fluxcd.io",
				Version: version,
			}),
			shared.BlobResource(shared.Resource{
				Name:    "validation",
				Data:    string(validationContent),
				Type:    "validator.mpas.ocm.software",
				Version: version,
			}),
		},
	}
}

func podinfoFrontend(t *testing.T, privateKey []byte, privateKeyName string, componentNameIdentifier string, testPath string, version string) setup.Component {
	t.Helper()

	configContent, err := os.ReadFile(filepath.Join(basePath, testPath, podinfoName, frontend, "config.yaml"))
	if err != nil {
		t.Fatal(errMsgReadFile, err)
	}

	readmeContent, err := os.ReadFile(filepath.Join(basePath, testPath, podinfoName, frontend, "README.md"))
	if err != nil {
		t.Fatal(errMsgReadFile, err)
	}

	manifestContent, err := os.ReadFile(filepath.Join(basePath, testPath, podinfoName, frontend, "manifests.tar"))
	if err != nil {
		t.Fatal(errMsgReadFile, err)
	}

	validationContent, err := os.ReadFile(filepath.Join(basePath, testPath, podinfoName, frontend, "validation.rego"))
	if err != nil {
		t.Fatal(errMsgReadFile, err)
	}

	return setup.Component{
		Component:  getComponent(privateKeyName, privateKey, componentNameIdentifier, podinfoFrontendComponentName, version),
		Repository: frontend,
		ComponentVersionModifications: []shared.ComponentModification{
			shared.BlobResource(shared.Resource{
				Name:    "config",
				Data:    string(configContent),
				Type:    "configdata.ocm.software",
				Version: version,
			}),
			shared.BlobResource(shared.Resource{
				Name:    "instructions",
				Data:    string(readmeContent),
				Type:    "PlainText",
				Version: version,
			}),
			shared.ImageRefResource("ghcr.io/stefanprodan/podinfo:6.2.0", shared.Resource{
				Name:    "image",
				Version: "6.2.0",
				Type:    "ociImage",
			}),
			shared.BlobResource(shared.Resource{
				Name:    "manifests",
				Data:    string(manifestContent),
				Type:    "kustomize.ocm.fluxcd.io",
				Version: version,
			}),
			shared.BlobResource(shared.Resource{
				Name:    "validation",
				Data:    string(validationContent),
				Type:    "validator.mpas.ocm.software",
				Version: version,
			}),
		},
	}
}

func podinfoRedis(t *testing.T, privateKey []byte, privateKeyName string, componentNameIdentifier string, testPath string, version string) setup.Component {
	t.Helper()

	configContent, err := os.ReadFile(filepath.Join(basePath, testPath, podinfoName, redis, "config.yaml"))
	if err != nil {
		t.Fatal(errMsgReadFile, err)
	}

	readmeContent, err := os.ReadFile(filepath.Join(basePath, testPath, podinfoName, redis, "README.md"))
	if err != nil {
		t.Fatal(errMsgReadFile, err)
	}

	manifestContent, err := os.ReadFile(filepath.Join(basePath, testPath, podinfoName, redis, "manifests.tar"))
	if err != nil {
		t.Fatal(errMsgReadFile, err)
	}

	validationContent, err := os.ReadFile(filepath.Join(basePath, testPath, podinfoName, redis, "validation.rego"))
	if err != nil {
		t.Fatal(errMsgReadFile, err)
	}

	return setup.Component{
		Component:  getComponent(privateKeyName, privateKey, componentNameIdentifier, redisComponentName, version),
		Repository: redis,
		ComponentVersionModifications: []shared.ComponentModification{
			shared.BlobResource(shared.Resource{
				Name:    "config",
				Data:    string(configContent),
				Type:    "configdata.ocm.software",
				Version: version,
			}),
			shared.BlobResource(shared.Resource{
				Name:    "instructions",
				Data:    string(readmeContent),
				Type:    "PlainText",
				Version: version,
			}),
			shared.ImageRefResource("redis:6.0.1", shared.Resource{
				Name:    "image",
				Version: "6.2.0",
				Type:    "ociImage",
			}),
			shared.BlobResource(shared.Resource{
				Name:    "manifests",
				Data:    string(manifestContent),
				Type:    "kustomize.ocm.fluxcd.io",
				Version: version,
			}),
			shared.BlobResource(shared.Resource{
				Name:    "validation",
				Data:    string(validationContent),
				Type:    "validator.mpas.ocm.software",
				Version: version,
			}),
		},
	}
}
func getComponent(privateKeyName string, privateKey []byte, componentNameIdentifier string, componentName string, version string) shared.Component {
	if len(privateKeyName) > 0 && privateKey != nil {
		return shared.Component{
			Name:    componentNamePrefix + componentNameIdentifier + componentName,
			Version: version,
			Sign: &shared.Sign{
				Name: privateKeyName,
				Key:  privateKey,
			},
		}
	}
	return shared.Component{
		Name:    componentNamePrefix + componentNameIdentifier + componentName,
		Version: version,
	}
}
