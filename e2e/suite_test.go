// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package e2e

import (
	"os"
	"testing"
	"time"

	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"

	"github.com/open-component-model/ocm-e2e-framework/shared"
)

var (
	testEnv         env.Environment
	kindClusterName string
	namespace       string
)

func TestMain(m *testing.M) {
	setupLog("starting e2e test suite")

	cfg, _ := envconf.NewFromFlags()
	testEnv = env.NewWithConfig(cfg)
	kindClusterName = envconf.RandomName("ocm-ctrl-e2e"+time.Now().Format("2006-01-02-15-04-05"), 32)
	namespace = "ocm-system"

	stopChannelRegistry := make(chan struct{}, 1)
	stopChannelGitea := make(chan struct{}, 1)

	testEnv.Setup(
		envfuncs.CreateKindCluster(kindClusterName),
		envfuncs.CreateNamespace(namespace),
		shared.StartGitServer(namespace),
		shared.InstallFlux("latest"),
		shared.RunTiltForControllers("ocm-controller"),
		shared.ForwardPortForAppName("registry", 5000, stopChannelRegistry),
		shared.ForwardPortForAppName("gitea", 3000, stopChannelGitea),
	)
	// testEnv.AfterEachTest(
	// 	shared.RemoveGitServer(namespace),
	// 	shared.ShutdownPortForward(stopChannelRegistry),
	// 	shared.ShutdownPortForward(stopChannelGitea),
	// 	envfuncs.DeleteNamespace(namespace),
	// 	envfuncs.DestroyKindCluster(kindClusterName),	)

	// testEnv.AfterEachTest(func(ctx context.Context, cfg *envconf.Config, t *testing.T)  {
	// 	shared.RemoveGitServer(namespace),
	// 	shared.ShutdownPortForward(stopChannelRegistry),
	// 	shared.ShutdownPortForward(stopChannelGitea),
	// 	envfuncs.DeleteNamespace(namespace),
	// 	envfuncs.DestroyKindCluster(kindClusterName),
	// })
	// // The deletion uses the typical c.Resources() object.
	// cfg.Client().Resources().Delete(ctx,&nsObj)

	// testEnv.Finish(
	// 	shared.RemoveGitServer(namespace),
	// 	shared.ShutdownPortForward(stopChannelRegistry),
	// 	shared.ShutdownPortForward(stopChannelGitea),
	// 	envfuncs.DeleteNamespace(namespace),
	// 	envfuncs.DestroyKindCluster(kindClusterName),
	// )
	os.Exit(testEnv.Run(m))
}
