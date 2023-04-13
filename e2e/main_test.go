// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package e2e

import (
	"context"
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-e2e-framework/shared/steps/setup"
)

var (
	testRepoName = "ocm-controller-test"
	cvManifest   = setup.File{
		RepoName:       testRepoName,
		SourceFilepath: "component_version.yaml",
		DestFilepath:   "apps/component_version.yaml",
	}
	resourceManifest = setup.File{
		RepoName:       testRepoName,
		SourceFilepath: "resource.yaml",
		DestFilepath:   "apps/resource.yaml",
	}
	localizationManifest = setup.File{
		RepoName:       testRepoName,
		SourceFilepath: "localization.yaml",
		DestFilepath:   "apps/localization.yaml",
	}
	configurationManifest = setup.File{
		RepoName:       testRepoName,
		SourceFilepath: "configuration.yaml",
		DestFilepath:   "apps/configuration.yaml",
	}
	deployerManifest = setup.File{
		RepoName:       testRepoName,
		SourceFilepath: "deployer.yaml",
		DestFilepath:   "apps/deployer.yaml",
	}
	manifests = []setup.File{
		cvManifest,
		resourceManifest,
		localizationManifest,
		configurationManifest,
		deployerManifest,
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
			checkIsCVReady(getYAMLField("component_version.yaml", "metadata.name"))).
		Assess("check that resource is ready",
			checkIsResourceReady(getYAMLField("resource.yaml", "metadata.name"))).
		Assess("check that localization is ready",
			checkIsLocalizationReady(getYAMLField("localization.yaml", "metadata.name"))).
		Assess("check that configuration is ready",
			checkIsConfigurationReady(getYAMLField("configuration.yaml", "metadata.name"))).
		Assess("check that flux deployer is ready",
			checkIsFluxDeployerReady(getYAMLField("deployer.yaml", "metadata.name")))

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
				}), wait.WithTimeout(time.Minute*1))

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
				}), wait.WithTimeout(time.Minute*1))

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
		}), wait.WithTimeout(time.Minute*1))

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
		}), wait.WithTimeout(time.Minute*1))

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
		}), wait.WithTimeout(time.Minute*1))

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
		}), wait.WithTimeout(time.Minute*1))

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
		}), wait.WithTimeout(time.Minute*1))

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
		}), wait.WithTimeout(time.Minute*1))

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
		}), wait.WithTimeout(time.Minute*1))

		if err != nil {
			t.Fatal(err)
		}

		return ctx
	}
}
