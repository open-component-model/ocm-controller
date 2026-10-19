package main

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/yaml"
)

func TestHelmIntervalJitterPercentage(t *testing.T) {
	helm, err := exec.LookPath("helm")
	if err != nil {
		t.Skip("helm is required for chart rendering tests")
	}
	for _, tt := range []struct {
		name  string
		value string
		want  string
	}{
		{name: "default", value: "{}", want: "5"},
		{name: "disabled", value: "manager: {intervalJitterPercentage: 0}", want: "0"},
		{name: "custom", value: "manager: {intervalJitterPercentage: 25}", want: "25"},
		{name: "largest", value: "manager: {intervalJitterPercentage: 99}", want: "99"},
		{name: "null uses CLI default", value: "manager: {intervalJitterPercentage: null}"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(helm, "template", "test", "./deploy", "--show-only", "templates/deployment_manager.yaml", "--values", "-")
			cmd.Stdin = strings.NewReader(tt.value)
			output, err := cmd.CombinedOutput()
			require.NoError(t, err, string(output))
			var deployment appsv1.Deployment
			require.NoError(t, yaml.Unmarshal(output, &deployment))
			var jitterFlags []string
			for _, arg := range deployment.Spec.Template.Spec.Containers[0].Args {
				if strings.HasPrefix(arg, "--interval-jitter-percentage=") {
					jitterFlags = append(jitterFlags, arg)
				}
			}
			if tt.want == "" {
				require.Empty(t, jitterFlags)
			} else {
				require.Equal(t, []string{"--interval-jitter-percentage=" + tt.want}, jitterFlags)
			}
		})
	}
}
