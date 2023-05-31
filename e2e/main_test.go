// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package e2e

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"github.com/open-component-model/ocm-e2e-framework/shared"
	"strconv"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1beta2"
	"github.com/fluxcd/pkg/apis/meta"
	fconditions "github.com/fluxcd/pkg/runtime/conditions"
	sourcev1 "github.com/fluxcd/source-controller/api/v1beta2"
	"github.com/google/go-containerregistry/pkg/crane"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-e2e-framework/shared/steps/setup"
)

var (
	testRepoName                     = "ocm-controller-test"
	testRepoSignedName               = "ocm-controller-signed-test"
	testRepoSignedUnsignedName       = "ocm-controller-signed-unsigned-test"
	timeoutDuration                  = time.Minute * 2
	TestOCMControllerPath            = "testOCMController/"
	TestSignedComponentsPath         = "testSignedOCIRegistryComponents/"
	TestSignedUnsignedComponentsPath = "testSignedUnsignedOCIRegistryComponents"
	RsaPubKeyName                    = "rsa"
	RsaInvalidPubKeyName             = "rsainvalid"
	cvFile                           = TestOCMControllerPath + "component_version.yaml"
	localizationFile                 = TestOCMControllerPath + "localization.yaml"
	resourceFile                     = TestOCMControllerPath + "resource.yaml"
	configurationfile                = TestOCMControllerPath + "configuration.yaml"
	deployerFile                     = TestOCMControllerPath + "deployer.yaml"
	cvFileSigned                     = TestSignedComponentsPath + "component_version.yaml"
	cvFileSignedUnsigned             = TestSignedUnsignedComponentsPath + "component_version.yaml"
	cvManifest                       = setup.File{
		RepoName:       testRepoName,
		SourceFilepath: cvFile,
		DestFilepath:   "apps/component_version.yaml",
	}
	resourceManifest = setup.File{
		RepoName:       testRepoName,
		SourceFilepath: resourceFile,
		DestFilepath:   "apps/resource.yaml",
	}
	localizationManifest = setup.File{
		RepoName:       testRepoName,
		SourceFilepath: localizationFile,
		DestFilepath:   "apps/localization.yaml",
	}
	configurationManifest = setup.File{
		RepoName:       testRepoName,
		SourceFilepath: configurationfile,
		DestFilepath:   "apps/configuration.yaml",
	}
	deployerManifest = setup.File{
		RepoName:       testRepoName,
		SourceFilepath: deployerFile,
		DestFilepath:   "apps/deployer.yaml",
	}
	cvManifestSigned = setup.File{
		RepoName:       testRepoSignedName,
		SourceFilepath: cvFileSigned,
		DestFilepath:   "apps/component_version.yaml",
	}
	resourceManifestSigned = setup.File{
		RepoName:       testRepoSignedName,
		SourceFilepath: resourceFile,
		DestFilepath:   "apps/resource.yaml",
	}
	localizationManifestSigned = setup.File{
		RepoName:       testRepoSignedName,
		SourceFilepath: localizationFile,
		DestFilepath:   "apps/localization.yaml",
	}
	configurationManifestSigned = setup.File{
		RepoName:       testRepoSignedName,
		SourceFilepath: configurationfile,
		DestFilepath:   "apps/configuration.yaml",
	}
	deployerManifestSigned = setup.File{
		RepoName:       testRepoSignedName,
		SourceFilepath: deployerFile,
		DestFilepath:   "apps/deployer.yaml",
	}
	cvManifestSignedUnsigned = setup.File{
		RepoName:       testRepoSignedUnsignedName,
		SourceFilepath: cvFileSignedUnsigned,
		DestFilepath:   "apps/component_version.yaml",
	}
	resourceManifestSignedUnsigned = setup.File{
		RepoName:       testRepoSignedUnsignedName,
		SourceFilepath: resourceFile,
		DestFilepath:   "apps/resource.yaml",
	}
	localizationManifestSignedUnsigned = setup.File{
		RepoName:       testRepoSignedUnsignedName,
		SourceFilepath: localizationFile,
		DestFilepath:   "apps/localization.yaml",
	}
	configurationManifestSignedUnsigned = setup.File{
		RepoName:       testRepoSignedUnsignedName,
		SourceFilepath: configurationfile,
		DestFilepath:   "apps/configuration.yaml",
	}
	deployerManifestSignedUnsigned = setup.File{
		RepoName:       testRepoSignedUnsignedName,
		SourceFilepath: deployerFile,
		DestFilepath:   "apps/deployer.yaml",
	}
	manifests = []setup.File{
		cvManifest,
		resourceManifest,
		localizationManifest,
		configurationManifest,
		deployerManifest,
	}
	manifestsSigned = []setup.File{
		cvManifestSigned,
		resourceManifestSigned,
		localizationManifestSigned,
		configurationManifestSigned,
		deployerManifestSigned,
	}
	manifestsSignedUnsigned = []setup.File{
		cvManifestSignedUnsigned,
		resourceManifestSignedUnsigned,
		localizationManifestSignedUnsigned,
		configurationManifestSignedUnsigned,
		deployerManifestSignedUnsigned,
	}
)

func TestOCMController(t *testing.T) {
	t.Log("running e2e ocm-controller tests")

	management := features.New("Configure Management Repository").
		Setup(setup.AddScheme(v1alpha1.AddToScheme)).
		Setup(setup.AddScheme(sourcev1.AddToScheme)).
		Setup(setup.AddScheme(kustomizev1.AddToScheme)).
		Setup(setup.AddGitRepository(testRepoName)).
		Setup(setup.AddFluxSyncForRepo(testRepoName, "apps/", namespace))

	manifests := features.New("Create Manifests").
		Setup(setup.AddFilesToGitRepository(manifests...)).
		Assess("check that component version is ready",
			checkIsCVReady(getYAMLField(cvFile, "metadata.name"))).
		Assess("check that resource is ready",
			checkIsResourceReady(getYAMLField(resourceFile, "metadata.name"))).
		Assess("check that localization is ready",
			checkIsLocalizationReady(getYAMLField(localizationFile, "metadata.name"))).
		Assess("check that configuration is ready",
			checkIsConfigurationReady(getYAMLField(configurationfile, "metadata.name"))).
		Assess("check that flux deployer is ready",
			checkIsFluxDeployerReady(getYAMLField(deployerFile, "metadata.name")))

	validation := features.New("Validate OCM Pipeline").
		Assess("check that backend deployment was localized",
			func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
				client, err := cfg.NewClient()
				if err != nil {
					t.Fail()
				}

				gr := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{Name: "backend", Namespace: "ocm-system"},
				}

				err = wait.For(conditions.New(client.Resources()).ResourceMatch(gr, func(object k8s.Object) bool {
					obj, ok := object.(*appsv1.Deployment)
					if !ok {
						return false
					}
					img := obj.Spec.Template.Spec.Containers[0].Image
					return strings.Contains(img, "ghcr.io/stefanprodan/podinfo")
				}), wait.WithTimeout(timeoutDuration))

				return ctx
			}).
		Assess("check that configmap was configured",
			func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
				client, err := cfg.NewClient()
				if err != nil {
					t.Fail()
				}

				gr := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: "backend-config", Namespace: "ocm-system"},
				}

				err = wait.For(conditions.New(client.Resources()).ResourceMatch(gr, func(object k8s.Object) bool {
					obj, ok := object.(*corev1.ConfigMap)
					if !ok {
						return false
					}
					return obj.Data["PODINFO_UI_MESSAGE"] == "This is a test message"
				}), wait.WithTimeout(timeoutDuration))

				return ctx
			})

	testEnv.Test(t,
		management.Feature(),
		manifests.Feature(),
		validation.Feature(),
	)
}

func checkIsCVReady(name string) features.Func {
	return func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		t.Helper()

		client, err := cfg.NewClient()
		if err != nil {
			t.Fail()
		}

		t.Logf("checking readiness of ComponentVersion with name: %s...", name)
		gr := &v1alpha1.ComponentVersion{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ocm-system"},
		}

		err = wait.For(conditions.New(client.Resources()).ResourceMatch(gr, func(object k8s.Object) bool {
			obj, ok := object.(*v1alpha1.ComponentVersion)
			if !ok {
				return false
			}
			return fconditions.IsTrue(obj, meta.ReadyCondition)
		}), wait.WithTimeout(timeoutDuration))

		if err != nil {
			t.Fatal(err)
		}

		return ctx
	}
}

func checkIsResourceReady(name string) features.Func {
	return func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		t.Helper()

		client, err := cfg.NewClient()
		if err != nil {
			t.Fail()
		}

		t.Logf("checking readiness of Resource with name: %s...", name)
		gr := &v1alpha1.Resource{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ocm-system"},
		}

		err = wait.For(conditions.New(client.Resources()).ResourceMatch(gr, func(object k8s.Object) bool {
			obj, ok := object.(*v1alpha1.Resource)
			if !ok {
				return false
			}
			return fconditions.IsTrue(obj, meta.ReadyCondition)
		}), wait.WithTimeout(timeoutDuration))

		if err != nil {
			t.Fatal(err)
		}

		return ctx
	}
}

func checkIsConfigurationReady(name string) features.Func {
	return func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		t.Helper()

		client, err := cfg.NewClient()
		if err != nil {
			t.Fail()
		}

		t.Logf("checking readiness of Configuration with name: %s...", name)
		gr := &v1alpha1.Configuration{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ocm-system"},
		}

		err = wait.For(conditions.New(client.Resources()).ResourceMatch(gr, func(object k8s.Object) bool {
			obj, ok := object.(*v1alpha1.Configuration)
			if !ok {
				return false
			}
			return fconditions.IsTrue(obj, meta.ReadyCondition)
		}), wait.WithTimeout(timeoutDuration))

		if err != nil {
			t.Fatal(err)
		}

		return ctx
	}
}
func checkIsLocalizationReady(name string) features.Func {
	return func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		t.Helper()

		client, err := cfg.NewClient()
		if err != nil {
			t.Fail()
		}

		t.Logf("checking readiness of Localization with name: %s...", name)
		gr := &v1alpha1.Localization{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ocm-system"},
		}

		err = wait.For(conditions.New(client.Resources()).ResourceMatch(gr, func(object k8s.Object) bool {
			obj, ok := object.(*v1alpha1.Localization)
			if !ok {
				return false
			}
			return fconditions.IsTrue(obj, meta.ReadyCondition)
		}), wait.WithTimeout(timeoutDuration))

		if err != nil {
			t.Fatal(err)
		}

		return ctx
	}
}
func checkIsFluxDeployerReady(name string) features.Func {
	return func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		t.Helper()

		client, err := cfg.NewClient()
		if err != nil {
			t.Fail()
		}

		t.Logf("checking readiness of Flux Deployer with name: %s...", name)
		gr := &v1alpha1.FluxDeployer{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ocm-system"},
		}

		err = wait.For(conditions.New(client.Resources()).ResourceMatch(gr, func(object k8s.Object) bool {
			obj, ok := object.(*v1alpha1.FluxDeployer)
			if !ok {
				return false
			}
			return fconditions.IsTrue(obj, meta.ReadyCondition)
		}), wait.WithTimeout(timeoutDuration))

		if err != nil {
			t.Fatal(err)
		}

		t.Logf("checking if oci repository %s is ready...", name)
		sourceGR := &sourcev1.OCIRepository{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ocm-system"},
		}

		err = wait.For(conditions.New(client.Resources()).ResourceMatch(sourceGR, func(object k8s.Object) bool {
			obj, ok := object.(*sourcev1.OCIRepository)
			if !ok {
				return false
			}
			return fconditions.IsTrue(obj, meta.ReadyCondition)
		}), wait.WithTimeout(timeoutDuration))

		if err != nil {
			t.Fatal(err)
		}

		t.Logf("checking if kustomization %s is ready...", name)
		kustomizeGR := &kustomizev1.Kustomization{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ocm-system"},
		}

		err = wait.For(conditions.New(client.Resources()).ResourceMatch(kustomizeGR, func(object k8s.Object) bool {
			obj, ok := object.(*kustomizev1.Kustomization)
			if !ok {
				return false
			}
			return fconditions.IsTrue(obj, meta.ReadyCondition)
		}), wait.WithTimeout(timeoutDuration))

		if err != nil {
			t.Fatal(err)
		}

		return ctx
	}
}

func TestComponentUploadToLocalOCIRegistry(t *testing.T) {
	t.Log("Test component-version transfer to local oci repository")

	setupComponent := createTestComponentVersion(t)

	validation := features.New("Validate if OCM Components are present in OCI Registry").
		Setup(setup.AddScheme(v1alpha1.AddToScheme)).
		Assess("Validate Component "+podinfoComponentName, checkRepositoryExistsInRegistry(podinfoComponentName)).
		Assess("Validate Component "+podinfoBackendComponentName, checkRepositoryExistsInRegistry(podinfoBackendComponentName)).
		Assess("Validate Component "+podinfoFrontendComponentName, checkRepositoryExistsInRegistry(podinfoFrontendComponentName)).
		Assess("Validate Component "+redisComponentName, checkRepositoryExistsInRegistry(redisComponentName))

	testEnv.Test(t,
		setupComponent.Feature(),
		validation.Feature(),
	)
}

func checkRepositoryExistsInRegistry(componentName string) features.Func {
	return func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		t.Helper()
		res, err := crane.Catalog(hostUrl + portSeparator + strconv.Itoa(registryPort))
		if err != nil {
			t.Fatal(err)
		}

		if !containsComponent(res, componentName) {
			t.Fail()
		}
		return ctx
	}
}
func containsComponent(craneCatalogRes []string, component string) bool {
	for _, returnedComponent := range craneCatalogRes {
		t := strings.Index(returnedComponent, "/")
		returnedComponent = returnedComponent[t+1:]
		if returnedComponent == component {
			return true
		}
	}
	return false
}

func TestRSASignedComponentUploadToLocalOCIRegistry(t *testing.T) {
	t.Log("Test signed component-version transfer to local oci repository")

	privateKey, publicKey, err := createRSAKeys()
	if err != nil {
		t.Fatal(err)
	}
	setupComponent := createTestComponentVersionSigned(t, privateKey)

	validation := features.New("Validate if signed OCM Components are present in OCI Registry").
		Setup(setup.AddScheme(v1alpha1.AddToScheme)).
		Setup(setup.AddGitRepository(testRepoSignedName)).
		Setup(setup.AddFilesToGitRepository(manifestsSigned...)).
		Setup(shared.CreateSecret(RsaPubKeyName, publicKey)).
		Assess("Validate Component "+podinfoComponentName, checkRepositoryExistsInRegistry(podinfoComponentName)).
		Assess("Validate Component "+podinfoBackendComponentName, checkRepositoryExistsInRegistry(podinfoBackendComponentName)).
		Assess("Validate Component "+podinfoFrontendComponentName, checkRepositoryExistsInRegistry(podinfoFrontendComponentName)).
		Assess("Validate Component "+redisComponentName, checkRepositoryExistsInRegistry(redisComponentName))

	testEnv.Test(t,
		setupComponent.Feature(),
		validation.Feature(),
	)
}

func createRSAKeys() ([]byte, []byte, error) {
	bitSize := 2048

	key, err := rsa.GenerateKey(rand.Reader, bitSize)
	if err != nil {
		panic(err)
	}

	pub := key.Public()

	// Encode private key to PKCS#1 ASN.1 PEM.
	keyPEM := pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(key),
		},
	)

	// Encode public key to PKCS#1 ASN.1 PEM.
	pubPEM := pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PUBLIC KEY",
			Bytes: x509.MarshalPKCS1PublicKey(pub.(*rsa.PublicKey)),
		},
	)

	return keyPEM, pubPEM, nil
}

func TestSignedUnsignedComponentUploadToLocalOCIRegistry(t *testing.T) {
	t.Log("Test signed component-version transfer to local oci repository")

	_, publicKey, err := createRSAKeys()
	if err != nil {
		t.Fatal(err)
	}
	privateKey, _, err := createRSAKeys()
	if err != nil {
		t.Fatal(err)
	}

	setupComponent := createTestComponentVersionSigned(t, privateKey)

	validation := features.New("Validate if OCM Components are present in OCI Registry").
		Setup(setup.AddScheme(v1alpha1.AddToScheme)).
		Setup(setup.AddGitRepository(testRepoSignedUnsignedName)).
		Setup(setup.AddFilesToGitRepository(manifestsSignedUnsigned...)).
		Setup(shared.CreateSecret(RsaInvalidPubKeyName, publicKey)).
		Assess("Validate Component "+podinfoComponentName, checkRepositoryExistsInRegistry(podinfoComponentName)).
		Assess("Validate Component "+podinfoBackendComponentName, checkRepositoryExistsInRegistry(podinfoBackendComponentName)).
		Assess("Validate Component "+podinfoFrontendComponentName, checkRepositoryExistsInRegistry(podinfoFrontendComponentName)).
		Assess("Validate Component "+redisComponentName, checkRepositoryExistsInRegistry(redisComponentName))

	testEnv.Test(t,
		setupComponent.Feature(),
		validation.Feature(),
	)
}
