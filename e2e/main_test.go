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
	"github.com/fluxcd/helm-controller/api/v2beta1"
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
	testRepoName                    = "ocm-controller-test"
	testRepoUnsignedName            = "ocm-controller-unsigned-test"
	testRepoSignedName              = "ocm-controller-signed-test"
	testRepoSignedInvalidName       = "ocm-controller-signed-invalid-test"
	timeoutDuration                 = time.Minute * 2
	TestOCMControllerPath           = "testOCMController/"
	TestUnsignedComponentsPath      = "testUnsignedOCIRegistryComponents/"
	TestSignedComponentsPath        = "testSignedOCIRegistryComponents/"
	TestSignedInvalidComponentsPath = "testSignedInvalidOCIRegistryComponents/"
	KeyName                         = "rsa"
	cvFile                          = "component_version.yaml"
	localizationFile                = "localization.yaml"
	resourceFile                    = "resource.yaml"
	configurationFile               = "configuration.yaml"
	deployerFile                    = "deployer.yaml"
	destinationPrefix               = "apps/"
)

func getManifests(testName string, gitRepositoryName string) []setup.File {
	cvManifest := setup.File{
		RepoName:       gitRepositoryName,
		SourceFilepath: testName + cvFile,
		DestFilepath:   destinationPrefix + cvFile,
	}
	resourceManifest := setup.File{
		RepoName:       gitRepositoryName,
		SourceFilepath: testName + resourceFile,
		DestFilepath:   destinationPrefix + resourceFile,
	}
	localizationManifest := setup.File{
		RepoName:       gitRepositoryName,
		SourceFilepath: testName + localizationFile,
		DestFilepath:   destinationPrefix + localizationFile,
	}
	configurationManifest := setup.File{
		RepoName:       gitRepositoryName,
		SourceFilepath: testName + configurationFile,
		DestFilepath:   destinationPrefix + configurationFile,
	}

	deployerManifest := setup.File{
		RepoName:       gitRepositoryName,
		SourceFilepath: testName + deployerFile,
		DestFilepath:   destinationPrefix + deployerFile,
	}

	return []setup.File{
		cvManifest,
		resourceManifest,
		localizationManifest,
		configurationManifest,
		deployerManifest,
	}
}
func TestOCMController(t *testing.T) {
	t.Log("running e2e ocm-controller tests")

	management := features.New("Configure Management Repository").
		Setup(setup.AddScheme(v1alpha1.AddToScheme)).
		Setup(setup.AddScheme(sourcev1.AddToScheme)).
		Setup(setup.AddScheme(kustomizev1.AddToScheme)).
		Setup(setup.AddGitRepository(testRepoName)).
		Setup(setup.AddFluxSyncForRepo(testRepoName, destinationPrefix, namespace))

	manifests := features.New("Create Manifests").
		Setup(setup.AddFilesToGitRepository(getManifests(TestOCMControllerPath, testRepoName)...)).
		Assess("check that component version is ready",
			checkCVConditionType(getYAMLField(TestOCMControllerPath+cvFile, "metadata.name"), meta.ReadyCondition)).
		Assess("check that component version is valid", checkCVReason(getYAMLField(TestOCMControllerPath+cvFile, "metadata.name"), meta.SucceededReason)).
		Assess("check that resource is ready",
			checkIsResourceReady(getYAMLField(TestOCMControllerPath+resourceFile, "metadata.name"))).
		Assess("check that localization is ready",
			checkIsLocalizationReady(getYAMLField(TestOCMControllerPath+localizationFile, "metadata.name"))).
		Assess("check that configuration is ready",
			checkIsConfigurationReady(getYAMLField(TestOCMControllerPath+configurationFile, "metadata.name"))).
		Assess("check that flux deployer is ready",
			checkIsFluxDeployerReady(getYAMLField(TestOCMControllerPath+deployerFile, "metadata.name")))

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

func checkCVConditionType(name string, conditionType string) features.Func {
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
			t.Log("conditionType: ", conditionType)
			return fconditions.IsTrue(obj, conditionType)
		}), wait.WithTimeout(timeoutDuration))

		t.Log("condition met")
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
			return fconditions.IsTrue(obj, meta.ReadyCondition) && reasonMatches(obj, meta.SucceededReason)
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
			return fconditions.IsTrue(obj, meta.ReadyCondition) && reasonMatches(obj, meta.SucceededReason)
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
			return fconditions.IsTrue(obj, meta.ReadyCondition) && reasonMatches(obj, meta.SucceededReason)
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
			return fconditions.IsTrue(obj, meta.ReadyCondition) && reasonMatches(obj, meta.SucceededReason)
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
			return fconditions.IsTrue(obj, meta.ReadyCondition) && reasonMatches(obj, meta.SucceededReason)
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
			return fconditions.IsTrue(obj, meta.ReadyCondition) && reasonMatches(obj, v2beta1.ReconciliationSucceededReason)
		}), wait.WithTimeout(timeoutDuration))

		if err != nil {
			t.Fatal(err)
		}

		return ctx
	}
}

func TestComponentUploadToLocalOCIRegistry(t *testing.T) {
	t.Log("Test component-version transfer to local oci repository")
	name := getYAMLField(TestUnsignedComponentsPath+cvFile, "metadata.name")
	componentNameIdentifier := "unsigned"
	setupComponent := createTestComponentVersion(t, "unsigned")
	validation := features.New("Validate if OCM Components are present in OCI Registry").
		Setup(setup.AddScheme(v1alpha1.AddToScheme)).
		Setup(setup.AddScheme(sourcev1.AddToScheme)).
		Setup(setup.AddScheme(kustomizev1.AddToScheme)).
		Setup(setup.AddGitRepository(testRepoUnsignedName)).
		Setup(setup.AddFilesToGitRepository(getManifests(TestUnsignedComponentsPath, testRepoUnsignedName)...)).
		Setup(setup.AddFluxSyncForRepo(testRepoUnsignedName, destinationPrefix, namespace)).
		Assess("Validate Component "+podinfoComponentName, checkRepositoryExistsInRegistry(componentNamePrefix+componentNameIdentifier+podinfoComponentName)).
		Assess("Validate Component "+podinfoBackendComponentName, checkRepositoryExistsInRegistry(componentNamePrefix+componentNameIdentifier+podinfoBackendComponentName)).
		Assess("Validate Component "+podinfoFrontendComponentName, checkRepositoryExistsInRegistry(componentNamePrefix+componentNameIdentifier+podinfoFrontendComponentName)).
		Assess("Validate Component "+redisComponentName, checkRepositoryExistsInRegistry(componentNamePrefix+componentNameIdentifier+redisComponentName)).
		Assess("Check that component version is ready", checkCVConditionType(name, meta.ReadyCondition)).
		Assess("Check that component version reoncile succeeded", checkCVReason(name, meta.SucceededReason))
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

		for _, returnedComponent := range res {
			fmt.Println("crane catalog", returnedComponent)
			fmt.Println("component", componentName)

			if strings.Contains(returnedComponent, componentName) {
				return ctx
			}
		}
		t.Fail()
		return ctx
	}
}

func TestRSASignedComponentUploadToLocalOCIRegistry(t *testing.T) {
	t.Log("Test signed component-version transfer to local oci repository")

	name := getYAMLField(TestSignedComponentsPath+cvFile, "metadata.name")
	componentNameIdentifier := "signed"
	privateKey, publicKey, err := createRSAKeys()
	if err != nil {
		t.Fatal(err)
	}

	setupComponent := createTestComponentVersionSigned(t, "Add signed components to component-version", privateKey, KeyName, publicKey, KeyName, componentNameIdentifier)
	validation := features.New("Validate if signed OCM Components are present in OCI Registry").
		Setup(setup.AddScheme(v1alpha1.AddToScheme)).
		Setup(setup.AddScheme(sourcev1.AddToScheme)).
		Setup(setup.AddScheme(kustomizev1.AddToScheme)).
		Setup(setup.AddGitRepository(testRepoSignedName)).
		Setup(setup.AddFilesToGitRepository(getManifests(TestSignedComponentsPath, testRepoSignedName)...)).
		Setup(setup.AddFluxSyncForRepo(testRepoSignedName, destinationPrefix, namespace)).
		Assess("Validate Component "+podinfoComponentName, checkRepositoryExistsInRegistry(componentNamePrefix+componentNameIdentifier+podinfoComponentName)).
		Assess("Validate Component "+podinfoBackendComponentName, checkRepositoryExistsInRegistry(componentNamePrefix+componentNameIdentifier+podinfoBackendComponentName)).
		Assess("Validate Component "+podinfoFrontendComponentName, checkRepositoryExistsInRegistry(componentNamePrefix+componentNameIdentifier+podinfoFrontendComponentName)).
		Assess("Validate Component "+redisComponentName, checkRepositoryExistsInRegistry(componentNamePrefix+componentNameIdentifier+redisComponentName))

	signatureVerification := features.New("Validate if signed OCM Components are present in OCI Registry").
		Assess("Check that component version is ready", checkCVConditionType(name, meta.ReadyCondition)).
		Assess("Check that component version signature was verified", checkCVReason(name, meta.SucceededReason)).
		Teardown(shared.DeleteSecret(KeyName))
	testEnv.Test(t,
		setupComponent.Feature(),
		validation.Feature(),
		signatureVerification.Feature(),
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

func checkCVReason(name string, reason string) features.Func {
	return func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		client, err := cfg.NewClient()
		if err != nil {
			t.Fatal(err)
		}

		gr := &v1alpha1.ComponentVersion{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ocm-system"},
		}

		err = wait.For(conditions.New(client.Resources()).ResourceMatch(gr, func(object k8s.Object) bool {
			obj, ok := object.(*v1alpha1.ComponentVersion)
			if !ok {
				return false
			}
			return reasonMatches(obj, reason)
		}), wait.WithTimeout(timeoutDuration))

		if err != nil {
			t.Fatal(err)
		}
		return ctx
	}
}
func reasonMatches(from fconditions.Getter, reason string) bool {
	conditions_ := from.GetConditions()
	if conditions_ == nil {
		return false
	}
	for _, condition := range conditions_ {
		if condition.Reason == reason {
			return true
		}
	}
	return false
}

func TestSignedInvalidComponentUploadToLocalOCIRegistry(t *testing.T) {
	t.Log("Test invalid signed component-version transfer to local oci repository")

	name := getYAMLField(TestSignedInvalidComponentsPath+cvFile, "metadata.name")
	componentNameIdentifier := "signedinvalid"

	privateKey, _, err := createRSAKeys()
	if err != nil {
		t.Fatal(err)
	}

	_, invalidPublicKey, err := createRSAKeys()
	if err != nil {
		t.Fatal(err)
	}

	setupComponent := createTestComponentVersionSigned(t, "Add signed invalid components to component-version", privateKey, KeyName, invalidPublicKey, KeyName, componentNameIdentifier)
	validation := features.New("Validate if invalid signed OCM Components are present in OCI Registry").
		Setup(setup.AddScheme(v1alpha1.AddToScheme)).
		Setup(setup.AddScheme(sourcev1.AddToScheme)).
		Setup(setup.AddScheme(kustomizev1.AddToScheme)).
		Setup(setup.AddGitRepository(testRepoSignedInvalidName)).
		Setup(setup.AddFilesToGitRepository(getManifests(TestSignedInvalidComponentsPath, testRepoSignedInvalidName)...)).
		Setup(setup.AddFluxSyncForRepo(testRepoSignedInvalidName, destinationPrefix, namespace)).
		Assess("Validate Component "+podinfoComponentName, checkRepositoryExistsInRegistry(componentNamePrefix+componentNameIdentifier+podinfoComponentName)).
		Assess("Validate Component "+podinfoBackendComponentName, checkRepositoryExistsInRegistry(componentNamePrefix+componentNameIdentifier+podinfoBackendComponentName)).
		Assess("Validate Component "+podinfoFrontendComponentName, checkRepositoryExistsInRegistry(componentNamePrefix+componentNameIdentifier+podinfoFrontendComponentName)).
		Assess("Validate Component "+redisComponentName, checkRepositoryExistsInRegistry(componentNamePrefix+componentNameIdentifier+redisComponentName))

	signatureVerification := features.New("Validate if signed OCM Components are present in OCI Registry").
		Assess("Check that component version is not ready", checkCVConditionType(name, meta.StalledCondition)).
		Assess("Check that component version signature verification failed", checkCVReason(name, v1alpha1.VerificationFailedReason))

	testEnv.Test(t,
		setupComponent.Feature(),
		validation.Feature(),
		signatureVerification.Feature(),
	)
}
