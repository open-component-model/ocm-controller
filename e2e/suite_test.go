// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package e2e

import (
	"os"
	"testing"

	"github.com/open-component-model/ocm-e2e-framework/shared"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
)

var (
	testEnv           env.Environment
	kindClusterName   string
	namespace         string
	registryPort      = 5000
	gitRepositoryPort = 3000
	hostUrl           = "localhost"
	portSeparator     = ":"
	localOciRegistry  = "registry"
	localGitService   = "gitea"
)

func TestMain(m *testing.M) {
	setupLog("starting e2e test suite")

	cfg, _ := envconf.NewFromFlags()
	testEnv = env.NewWithConfig(cfg)
	kindClusterName = envconf.RandomName("ocm-ctrl-e2e", 32)
	namespace = "ocm-system"

	stopChannelRegistry := make(chan struct{}, 1)
	stopChannelGitea := make(chan struct{}, 1)

	if err := os.Setenv("REGISTRY_DISABLE_HTTPS", "1"); err != nil {
		os.Exit(1)
	}

	testEnv.Setup(
		envfuncs.CreateKindCluster(kindClusterName),
		envfuncs.CreateNamespace(namespace),
		shared.StartGitServer(namespace),
		shared.InstallFlux("latest"),
		shared.RunTiltForControllers("ocm-controller"),
		shared.ForwardPortForAppName(localOciRegistry, registryPort, stopChannelRegistry),
		shared.ForwardPortForAppName(localGitService, gitRepositoryPort, stopChannelGitea),
	)

	testEnv.Finish(
		shared.RemoveGitServer(namespace),
		shared.ShutdownPortForward(stopChannelRegistry),
		shared.ShutdownPortForward(stopChannelGitea),
		envfuncs.DeleteNamespace(namespace),
		envfuncs.DestroyKindCluster(kindClusterName),
	)

	os.Exit(testEnv.Run(m))
}
