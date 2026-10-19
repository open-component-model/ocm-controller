package main

import (
	"flag"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/yaml"
)

func TestValidateIntervalJitterPercentage(t *testing.T) {
	for _, value := range []uint{0, 5, 99} {
		require.NoError(t, validateIntervalJitterPercentage(value))
	}
	for _, value := range []uint{100, 255, 1000, ^uint(0)} {
		require.Error(t, validateIntervalJitterPercentage(value))
	}
}

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
		{name: "negative", value: "manager: {intervalJitterPercentage: -1}"},
		{name: "too large", value: "manager: {intervalJitterPercentage: 100}"},
		{name: "overflow", value: "manager: {intervalJitterPercentage: 18446744073709551616}"},
		{name: "fractional", value: "manager: {intervalJitterPercentage: 2.5}"},
		{name: "boolean", value: "manager: {intervalJitterPercentage: true}"},
		{name: "string", value: "manager: {intervalJitterPercentage: invalid}"},
		{name: "null uses default", value: "manager: {intervalJitterPercentage: null}", want: "5"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(helm, "template", "test", "./deploy", "--show-only", "templates/deployment_manager.yaml", "--values", "-")
			cmd.Stdin = strings.NewReader(tt.value)
			output, err := cmd.CombinedOutput()
			if tt.want == "" {
				require.Error(t, err, string(output))
				require.Contains(t, string(output), "manager.intervalJitterPercentage must be an integer between 0 and 99")
				return
			}
			require.NoError(t, err, string(output))
			var deployment appsv1.Deployment
			require.NoError(t, yaml.Unmarshal(output, &deployment))
			require.Contains(t, deployment.Spec.Template.Spec.Containers[0].Args, "--interval-jitter-percentage="+tt.want)
		})
	}
}

func TestIntervalJitterFlag(t *testing.T) {
	for _, tt := range []struct {
		name string
		args []string
		want uint
	}{
		{name: "default", want: 5},
		{name: "disabled", args: []string{"--interval-jitter-percentage=0"}},
		{name: "custom", args: []string{"--interval-jitter-percentage=25"}, want: 25},
	} {
		t.Run(tt.name, func(t *testing.T) {
			flags := flag.NewFlagSet("test", flag.ContinueOnError)
			var percentage uint
			bindIntervalJitterFlag(flags, &percentage)
			require.NoError(t, flags.Parse(tt.args))
			require.NoError(t, validateIntervalJitterPercentage(percentage))
			require.Equal(t, tt.want, percentage)
		})
	}
}
