// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
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
		dumpControllerLogs(namespace),
		shared.RemoveGitServer(namespace),
		shared.ShutdownPortForward(stopChannelRegistry),
		shared.ShutdownPortForward(stopChannelGitea),
		envfuncs.DeleteNamespace(namespace),
		envfuncs.DestroyKindCluster(kindClusterName),
	)

	os.Exit(testEnv.Run(m))
}

func dumpControllerLogs(n string) env.Func {
	return func(ctx context.Context, config *envconf.Config) (context.Context, error) {
		fmt.Println("dumping logs")
		// kubectl logs `k get pods --template '{{range .items}}{{.metadata.name}}{{end}}' --selector=app=ocm-controller -n ocm-system` -n ocm-system -f
		cmd := exec.CommandContext(ctx, "kubectl", "get", "pods", "--template", "'{{range .items}}{{.metadata.name}}{{end}}'", "--selector=app=ocm-controller", "-n", "ocm-system")
		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Println("ERROR 1: ", err, string(output))
			return ctx, fmt.Errorf("failed to get pods: %w", err)
		}

		pod := strings.ReplaceAll(string(output), "'", "")
		fmt.Println("getting logs for pod: ", pod)

		cmd = exec.CommandContext(ctx, "kubectl", "logs", pod, "-n", "ocm-system")
		output, err = cmd.CombinedOutput()
		if err != nil {
			fmt.Println("ERROR 2: ", err, string(output))
			return ctx, fmt.Errorf("failed to gather logs from pod %s: %w", string(output), err)
		}

		fmt.Println(string(output))

		return ctx, nil
	}
}
