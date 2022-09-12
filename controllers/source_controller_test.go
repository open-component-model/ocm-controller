package controllers

import (
	"path/filepath"
	"testing"

	ocmcontrollerv1 "github.com/open-component-model/ocm-controller/api/v1"
	"github.com/stretchr/testify/assert"
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

func setup(t *testing.T) {

	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	assert.NoError(t, err)

	err = ocmcontrollerv1.AddToScheme(scheme.Scheme)
	assert.NoError(t, err)

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	assert.NoError(t, err)
	assert.NotNil(t, k8sClient)
}

func teardown(t *testing.T) {
	err := testEnv.Stop()
	assert.NoError(t, err)
}

func TestSourceController(t *testing.T) {
	setup(t)
	defer teardown(t)

}
