//go:build e2e

package e2e

import (
	"os"
	"testing"

	"sigs.k8s.io/e2e-framework/klient/conf"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"

	"github.com/open-component-model/ocm-e2e-framework/shared"
)

var (
	testEnv           env.Environment
	ocmNamespace      string
	registryPort      = 5000
	gitRepositoryPort = 3000
	hostUrl           = "localhost"
	portSeparator     = ":"
	localOciRegistry  = "registry"
	localGitService   = "gitea"
)

func TestMain(m *testing.M) {
	setupLog("starting e2e test suite")

	path := conf.ResolveKubeConfigFile()
	cfg := envconf.NewWithKubeConfig(path)
	testEnv = env.NewWithConfig(cfg)
	ocmNamespace = "ocm-system"

	stopChannelRegistry := make(chan struct{}, 1)
	stopChannelGitea := make(chan struct{}, 1)

	testEnv.Setup(
		envfuncs.CreateNamespace(ocmNamespace),
		shared.StartGitServer(ocmNamespace),
		shared.InstallFlux("latest"),
		shared.RunTiltForControllers("ocm-controller"),
		shared.ForwardPortForAppName(localOciRegistry, registryPort, stopChannelRegistry),
		shared.ForwardPortForAppName(localGitService, gitRepositoryPort, stopChannelGitea),
	)

	testEnv.Finish(
		shared.RemoveGitServer(ocmNamespace),
		shared.ShutdownPortForward(stopChannelRegistry),
		shared.ShutdownPortForward(stopChannelGitea),
		envfuncs.DeleteNamespace(ocmNamespace),
	)

	os.Exit(testEnv.Run(m))
}
