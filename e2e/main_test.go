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
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1beta2"
	"github.com/fluxcd/pkg/apis/meta"
	fconditions "github.com/fluxcd/pkg/runtime/conditions"
	sourcev1 "github.com/fluxcd/source-controller/api/v1beta2"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/open-component-model/ocm-e2e-framework/shared"
	"github.com/open-component-model/ocm-e2e-framework/shared/steps/setup"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
)

var (
	timeoutDuration            = time.Minute * 2
	testRepoName               = "ocm-controller-test"
	testRepoSignedName         = "ocm-controller-signed-test"
	testRepoHelmName           = "ocm-controller-helm-test"
	testHelmChartBasedResource = "testHelmChartResource"
	testOCMControllerPath      = "testOCMController"
	testSignedComponentsPath   = "testSignedOCIRegistryComponents"
	keyName                    = "rsa"
	cvFile                     = "component_version.yaml"
	localizationFile           = "localization.yaml"
	resourceFile               = "resource.yaml"
	configurationFile          = "configuration.yaml"
	deployerFile               = "deployer.yaml"
	destinationPrefix          = "apps/"
	identifier                 = "metadata.name"
	version1                   = "1.0.0"
)

func TestOCMController(t *testing.T) {
	t.Log("running e2e ocm-controller tests")
	componentNameIdentifier := "pipeline"

	setupComponent := createTestComponentVersionUnsigned(t, componentNameIdentifier, testOCMControllerPath, version1)

	management := features.New("Configure Management Repository").
		Setup(setup.AddScheme(v1alpha1.AddToScheme)).
		Setup(setup.AddScheme(sourcev1.AddToScheme)).
		Setup(setup.AddScheme(kustomizev1.AddToScheme)).
		Setup(setup.AddGitRepository(testRepoName)).
		Setup(setup.AddFluxSyncForRepo(testRepoName, destinationPrefix, ocmNamespace))

	cvName := getYAMLField(filepath.Join(testOCMControllerPath, cvFile), identifier)

	validateRegistry := features.New("Validate if OCM Components are present in OCI Registry").
		Assess("Validate Component "+podinfoComponentName, checkRepositoryExistsInRegistry(componentNamePrefix+componentNameIdentifier+podinfoComponentName)).
		Assess("Validate Component "+podinfoBackendComponentName, checkRepositoryExistsInRegistry(componentNamePrefix+componentNameIdentifier+podinfoBackendComponentName)).
		Assess("Validate Component "+podinfoFrontendComponentName, checkRepositoryExistsInRegistry(componentNamePrefix+componentNameIdentifier+podinfoFrontendComponentName)).
		Assess("Validate Component "+redisComponentName, checkRepositoryExistsInRegistry(componentNamePrefix+componentNameIdentifier+redisComponentName))

	componentVersion := features.New("Create Manifests").
		Setup(setup.AddFilesToGitRepository(getManifests(testOCMControllerPath, testRepoName)...)).
		Assess("check that component version "+cvName+" is ready and valid", checkIsComponentVersionReady(cvName, ocmNamespace))

	componentDescriptor := features.New("Check component-descriptors exist").
		Assess("check that component version "+cvName+" has Component Descriptors", checkCompDescriptorsExistForCompVersion(cvName, componentNamePrefix+componentNameIdentifier))

	validationManifestsBackend := checkCustomResourcesReadiness(backend)
	validationManifestsFrontend := checkCustomResourcesReadiness(frontend)
	validationManifestsRedis := checkCustomResourcesReadiness(redis)

	validationDeploymentBackend := checkDeploymentReadiness("backend", "ghcr.io/stefanprodan/podinfo")
	validationDeploymentFrontend := checkDeploymentReadiness("frontend", "ghcr.io/stefanprodan/podinfo")
	validationDeploymentRedis := checkDeploymentReadiness("redis", "redis")

	validationConfigMapBackend := checkConfigMapReadiness("backend-config", "This is a test message Pipeline Backend")
	validationConfigMapFrontend := checkConfigMapReadiness("frontend-config", "This is a test message Pipeline Frontend")

	testEnv.Test(t,
		setupComponent.Feature(),
		management.Feature(),
		validateRegistry.Feature(),
		componentVersion.Feature(),
		componentDescriptor.Feature(),
		validationManifestsBackend.Feature(),
		validationManifestsFrontend.Feature(),
		validationManifestsRedis.Feature(),
		validationDeploymentBackend.Feature(),
		validationDeploymentFrontend.Feature(),
		validationDeploymentRedis.Feature(),
		validationConfigMapBackend.Feature(),
		validationConfigMapFrontend.Feature(),
	)
}

func TestSignedComponentUploadToLocalOCIRegistry(t *testing.T) {
	t.Log("Test signed component-version transfer to local oci repository")

	cvName := getYAMLField(filepath.Join(testSignedComponentsPath, cvFile), identifier)
	componentNameIdentifier := "signed"
	privateKey, publicKey, err := createRSAKeys()
	if err != nil {
		t.Fatal(err)
	}

	_, invalidPublicKey, err := createRSAKeys()
	if err != nil {
		t.Fatal(err)
	}

	setupComponent := createTestComponentVersionSigned(t, "Add signed components to component-version", privateKey, keyName, invalidPublicKey, componentNameIdentifier, testSignedComponentsPath, version1)
	validation := features.New("Validate if signed OCM Components are present in OCI Registry").
		Setup(setup.AddScheme(v1alpha1.AddToScheme)).
		Setup(setup.AddScheme(sourcev1.AddToScheme)).
		Setup(setup.AddScheme(kustomizev1.AddToScheme)).
		Setup(setup.AddGitRepository(testRepoSignedName)).
		Setup(setup.AddFilesToGitRepository(getManifests(testSignedComponentsPath, testRepoSignedName)...)).
		Setup(setup.AddFluxSyncForRepo(testRepoSignedName, destinationPrefix, ocmNamespace)).
		Assess("Validate Component "+podinfoComponentName, checkRepositoryExistsInRegistry(componentNamePrefix+componentNameIdentifier+podinfoComponentName)).
		Assess("Validate Component "+podinfoBackendComponentName, checkRepositoryExistsInRegistry(componentNamePrefix+componentNameIdentifier+podinfoBackendComponentName)).
		Assess("Validate Component "+podinfoFrontendComponentName, checkRepositoryExistsInRegistry(componentNamePrefix+componentNameIdentifier+podinfoFrontendComponentName)).
		Assess("Validate Component "+redisComponentName, checkRepositoryExistsInRegistry(componentNamePrefix+componentNameIdentifier+redisComponentName)).
		Teardown(shared.DeleteSecret(keyName))

	signatureVerification := features.New("Validate if signed Component Versions of OCM Components exist").
		WithStep("create valid rsa key secret", 1, shared.CreateSecret(keyName, map[string][]byte{keyName: publicKey}, nil, "")).
		Assess("Check that component version "+cvName+" is ready and signature was verified", checkIsComponentVersionReady(cvName, ocmNamespace)).
		Teardown(shared.DeleteSecret(keyName))

	testEnv.Test(t,
		setupComponent.Feature(),
		validation.Feature(),
		signatureVerification.Feature(),
	)
}

func checkIsComponentVersionReady(name, namespace string) features.Func {
	return checkObjectCondition(&v1alpha1.ComponentVersion{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
	}, func(obj fconditions.Getter) bool {
		return fconditions.IsTrue(obj, meta.ReadyCondition) && reasonMatches(obj, meta.SucceededReason)
	})
}

func checkIsComponentVersionFailed(name string) features.Func {
	return checkObjectCondition(&v1alpha1.ComponentVersion{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ocm-system"},
	}, func(obj fconditions.Getter) bool {
		return fconditions.IsFalse(obj, meta.ReadyCondition) && reasonMatches(obj, v1alpha1.VerificationFailedReason)
	})
}

func checkIsResourceReady(name string) features.Func {
	return checkObjectCondition(&v1alpha1.Resource{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ocm-system"},
	}, func(obj fconditions.Getter) bool {
		return fconditions.IsTrue(obj, meta.ReadyCondition) && reasonMatches(obj, meta.SucceededReason)
	})
}

func checkIsConfigurationReady(name string) features.Func {
	return checkObjectCondition(&v1alpha1.Configuration{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ocm-system"},
	}, func(obj fconditions.Getter) bool {
		return fconditions.IsTrue(obj, meta.ReadyCondition) && reasonMatches(obj, meta.SucceededReason)
	})
}

func checkIsLocalizationReady(name string) features.Func {
	return checkObjectCondition(&v1alpha1.Localization{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ocm-system"},
	}, func(obj fconditions.Getter) bool {
		return fconditions.IsTrue(obj, meta.ReadyCondition) && reasonMatches(obj, meta.SucceededReason)
	})
}

func checkObjectCondition(conditionObject k8s.Object, condition func(getter fconditions.Getter) bool) features.Func {
	return func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		t.Helper()

		client, err := cfg.NewClient()
		if err != nil {
			t.Fail()
		}

		t.Logf("checking readiness of object with name: %s", conditionObject.GetName())

		err = wait.For(conditions.New(client.Resources()).ResourceMatch(conditionObject, func(object k8s.Object) bool {
			obj, ok := object.(fconditions.Getter)
			if !ok {
				return false
			}
			return condition(obj)
		}), wait.WithTimeout(timeoutDuration))

		if err != nil {
			t.Fatal(err)
		}

		return ctx
	}
}

func getManifests(testName string, gitRepositoryName string) []setup.File {
	cvManifest := setup.File{
		RepoName:       gitRepositoryName,
		SourceFilepath: filepath.Join(testName, cvFile),
		DestFilepath:   destinationPrefix + testName + cvFile,
	}
	resourceManifestBackend := setup.File{
		RepoName:       gitRepositoryName,
		SourceFilepath: filepath.Join(testName, podinfoName, backend, resourceFile),
		DestFilepath:   destinationPrefix + testName + backend + resourceFile,
	}
	localizationManifestBackend := setup.File{
		RepoName:       gitRepositoryName,
		SourceFilepath: filepath.Join(testName, podinfoName, backend, localizationFile),
		DestFilepath:   destinationPrefix + testName + backend + localizationFile,
	}
	configurationManifestBackend := setup.File{
		RepoName:       gitRepositoryName,
		SourceFilepath: filepath.Join(testName, podinfoName, backend, configurationFile),
		DestFilepath:   destinationPrefix + testName + backend + configurationFile,
	}
	deployerManifestBackend := setup.File{
		RepoName:       gitRepositoryName,
		SourceFilepath: filepath.Join(testName, podinfoName, backend, deployerFile),
		DestFilepath:   destinationPrefix + testName + backend + deployerFile,
	}
	resourceManifestFrontend := setup.File{
		RepoName:       gitRepositoryName,
		SourceFilepath: filepath.Join(testName, podinfoName, frontend, resourceFile),
		DestFilepath:   destinationPrefix + testName + frontend + resourceFile,
	}
	localizationManifestFrontend := setup.File{
		RepoName:       gitRepositoryName,
		SourceFilepath: filepath.Join(testName, podinfoName, frontend, localizationFile),
		DestFilepath:   destinationPrefix + testName + frontend + localizationFile,
	}
	configurationManifestFrontend := setup.File{
		RepoName:       gitRepositoryName,
		SourceFilepath: filepath.Join(testName, podinfoName, frontend, configurationFile),
		DestFilepath:   destinationPrefix + testName + frontend + configurationFile,
	}
	deployerManifestFrontend := setup.File{
		RepoName:       gitRepositoryName,
		SourceFilepath: filepath.Join(testName, podinfoName, frontend, deployerFile),
		DestFilepath:   destinationPrefix + testName + frontend + deployerFile,
	}
	resourceManifestRedis := setup.File{
		RepoName:       gitRepositoryName,
		SourceFilepath: filepath.Join(testName, podinfoName, redis, resourceFile),
		DestFilepath:   destinationPrefix + testName + redis + resourceFile,
	}
	localizationManifestRedis := setup.File{
		RepoName:       gitRepositoryName,
		SourceFilepath: filepath.Join(testName, podinfoName, redis, localizationFile),
		DestFilepath:   destinationPrefix + testName + redis + localizationFile,
	}
	configurationManifestRedis := setup.File{
		RepoName:       gitRepositoryName,
		SourceFilepath: filepath.Join(testName, podinfoName, redis, configurationFile),
		DestFilepath:   destinationPrefix + testName + redis + configurationFile,
	}
	deployerManifestRedis := setup.File{
		RepoName:       gitRepositoryName,
		SourceFilepath: filepath.Join(testName, podinfoName, redis, deployerFile),
		DestFilepath:   destinationPrefix + testName + redis + deployerFile,
	}

	return []setup.File{
		cvManifest,
		resourceManifestBackend,
		localizationManifestBackend,
		configurationManifestBackend,
		deployerManifestBackend,
		resourceManifestFrontend,
		localizationManifestFrontend,
		configurationManifestFrontend,
		deployerManifestFrontend,
		resourceManifestRedis,
		localizationManifestRedis,
		configurationManifestRedis,
		deployerManifestRedis,
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
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ocmNamespace},
		}

		err = wait.For(conditions.New(client.Resources()).ResourceMatch(gr, func(object k8s.Object) bool {
			obj, ok := object.(*v1alpha1.FluxDeployer)
			if !ok {
				return false
			}
			return fconditions.IsTrue(obj, meta.ReadyCondition) && reasonMatches(obj, meta.SucceededReason)
		}), wait.WithTimeout(timeoutDuration))

		if err != nil {
			t.Fatal(err)
		}

		t.Logf("checking if oci repository %s is ready...", name)
		sourceGR := &sourcev1.OCIRepository{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ocmNamespace},
		}

		err = wait.For(conditions.New(client.Resources()).ResourceMatch(sourceGR, func(object k8s.Object) bool {
			obj, ok := object.(*sourcev1.OCIRepository)
			if !ok {
				return false
			}
			return fconditions.IsTrue(obj, meta.ReadyCondition) && reasonMatches(obj, meta.SucceededReason)
		}), wait.WithTimeout(timeoutDuration))

		if err != nil {
			t.Fatal(err)
		}

		t.Logf("checking if kustomization %s is ready...", name)
		kustomizeGR := &kustomizev1.Kustomization{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ocmNamespace},
		}

		err = wait.For(conditions.New(client.Resources()).ResourceMatch(kustomizeGR, func(object k8s.Object) bool {
			obj, ok := object.(*kustomizev1.Kustomization)
			if !ok {
				return false
			}
			return fconditions.IsTrue(obj, meta.ReadyCondition) && reasonMatches(obj, "ReconciliationSucceeded")
		}), wait.WithTimeout(timeoutDuration))

		if err != nil {
			t.Fatal(err)
		}

		return ctx
	}
}

func checkRepositoryExistsInRegistry(componentName string) features.Func {
	return func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		t.Helper()
		res, err := crane.Catalog(hostUrl + portSeparator + strconv.Itoa(registryPort))
		if err != nil {
			t.Fatal(err)
		}

		for _, returnedComponent := range res {
			t.Log("crane catalog", returnedComponent)
			if strings.Contains(returnedComponent, componentName) {
				return ctx
			}
		}
		t.Fail()
		return ctx
	}
}

func checkCompDescriptorsExistForCompVersion(componentVersionName string, componentNamePrefix string) features.Func {
	return func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		cdNameSeparator := "-"
		client, err := cfg.NewClient()
		if err != nil {
			t.Fatal(err)
		}
		gr := &v1alpha1.ComponentVersion{
			ObjectMeta: metav1.ObjectMeta{Name: componentVersionName, Namespace: ocmNamespace},
		}
		err = client.Resources().Get(ctx, componentVersionName, ocmNamespace, gr)
		if err != nil {
			t.Fatal(err)
		}

		cdName := strings.Join([]string{componentNamePrefix, podinfoName, version1}, cdNameSeparator)
		cdNameNested := []string{
			strings.Join([]string{componentNamePrefix, podinfoName, backend, version1}, cdNameSeparator),
			strings.Join([]string{componentNamePrefix, podinfoName, frontend, version1}, cdNameSeparator),
			strings.Join([]string{componentNamePrefix, redis, version1}, cdNameSeparator),
		}

		if !strings.Contains(gr.Status.ComponentDescriptor.ComponentDescriptorRef.Name, cdName) {
			t.Fatal(fmt.Sprintf("Component Descriptor %s does not exist for Component Version: %s", cdName, componentVersionName))
		}

		for index, compDescRef := range gr.Status.ComponentDescriptor.References {
			if !strings.Contains(compDescRef.ComponentDescriptorRef.Name, cdNameNested[index]) {
				t.Fatal(fmt.Sprintf("Nested Component Descriptor %s not found, expected %s", compDescRef.ComponentDescriptorRef.Name, cdNameNested[index]))
			}
		}
		return ctx
	}
}

func checkCustomResourcesReadiness(path string) *features.FeatureBuilder {
	return features.New("Check if Manifests are Ready").
		Assess("check that "+path+" resource is ready",
			checkIsResourceReady(getYAMLField(filepath.Join(testOCMControllerPath, podinfoName, path, resourceFile), identifier))).
		Assess("check that "+path+" localization is ready",
			checkIsLocalizationReady(getYAMLField(filepath.Join(testOCMControllerPath, podinfoName, path, localizationFile), identifier))).
		Assess("check that "+path+" configuration is ready",
			checkIsConfigurationReady(getYAMLField(filepath.Join(testOCMControllerPath, podinfoName, path, configurationFile), identifier))).
		Assess("check that "+path+" flux deployer is ready",
			checkIsFluxDeployerReady(getYAMLField(filepath.Join(testOCMControllerPath, podinfoName, path, deployerFile), identifier)))
}

func checkDeploymentReadiness(deploymentName string, imageName string) *features.FeatureBuilder {
	return features.New("Validate OCM Pipeline: Deployment").
		Assess("check that deployment "+deploymentName+" is ready",
			func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
				client, err := cfg.NewClient()
				if err != nil {
					t.Fatal(err)
				}
				gr := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{Name: deploymentName, Namespace: ocmNamespace},
				}
				err = wait.For(conditions.New(client.Resources()).ResourceMatch(gr, func(object k8s.Object) bool {
					obj, ok := object.(*appsv1.Deployment)
					if !ok {
						return false
					}
					img := obj.Spec.Template.Spec.Containers[0].Image
					return strings.Contains(img, imageName)
				}), wait.WithTimeout(timeoutDuration))
				if err != nil {
					t.Fatal(err)
				}
				return ctx
			})
}

func checkConfigMapReadiness(configmapName string, message string) *features.FeatureBuilder {
	return features.New("Validate OCM Pipeline: ConfigMap").
		Assess("check that configmap "+configmapName+" was configured",
			func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
				client, err := cfg.NewClient()
				if err != nil {
					t.Fatal(err)
				}
				gr := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: configmapName, Namespace: ocmNamespace},
				}
				err = wait.For(conditions.New(client.Resources()).ResourceMatch(gr, func(object k8s.Object) bool {
					obj, ok := object.(*corev1.ConfigMap)
					if !ok {
						return false
					}
					return obj.Data["PODINFO_UI_MESSAGE"] == message
				}), wait.WithTimeout(timeoutDuration))
				if err != nil {
					t.Fatal(err)
				}
				return ctx
			})
}

func createRSAKeys() ([]byte, []byte, error) {
	bitSize := 2048

	key, err := rsa.GenerateKey(rand.Reader, bitSize)
	if err != nil {
		return nil, nil, err
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

func reasonMatches(from fconditions.Getter, reason string) bool {
	conditions_ := from.GetConditions()
	for _, condition := range conditions_ {
		if condition.Reason == reason {
			return true
		}
	}
	return false
}
