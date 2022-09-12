package controllers

import (
	"fmt"
	"path/filepath"
	"testing"

	ocmcontrollerv1 "github.com/open-component-model/ocm-controller/api/v1"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var (
	cfg       *rest.Config
	k8sClient client.Client
	testEnv   *envtest.Environment
)

func setup() {
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	// cfg is defined in this file globally.
	var err error
	cfg, err = testEnv.Start()
	if err != nil {
		panic(fmt.Errorf("failed to start test environment: %w", err))
	}

	if err = ocmcontrollerv1.AddToScheme(scheme.Scheme); err != nil {
		panic(fmt.Errorf("failed to add scheme: %w", err))
	}

	//+kubebuilder:scaffold:scheme
	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		panic(fmt.Errorf("failed to create client: %w", err))
	}
}

func teardown() {
	if err := testEnv.Stop(); err != nil {
		panic(fmt.Errorf("failed to stop test environment: %w", err))
	}
}

func TestMain(m *testing.M) {
	setup()
	defer teardown()
	m.Run()
}
