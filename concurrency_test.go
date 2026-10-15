package main

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/yaml"
)

func TestParseControllerConcurrency(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  map[string]int
	}{
		{name: "omitted"},
		{name: "empty", value: "  "},
		{
			name:  "partial with whitespace",
			value: " componentVersion = 4, resource=2 ",
			want: map[string]int{
				"ComponentVersion.delivery.ocm.software": 4,
				"Resource.delivery.ocm.software":         2,
			},
		},
		{
			name:  "all controllers",
			value: "componentVersion=2,resource=3,snapshot=4,localization=5,configuration=6,fluxDeployer=7",
			want: map[string]int{
				"ComponentVersion.delivery.ocm.software": 2,
				"Resource.delivery.ocm.software":         3,
				"Snapshot.delivery.ocm.software":         4,
				"Localization.delivery.ocm.software":     5,
				"Configuration.delivery.ocm.software":    6,
				"FluxDeployer.delivery.ocm.software":     7,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseControllerConcurrency(tt.value)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestParseControllerConcurrencyInvalid(t *testing.T) {
	for _, value := range []string{
		"unknown=2", "resource", "resource=", "=2", "resource=0",
		"resource=-1", "resource=1.5", "resource=two", "resource=2=3",
		"resource=2,resource=3", "resource=2,", "resource=999999999999999999999999",
	} {
		t.Run(value, func(t *testing.T) {
			_, err := parseControllerConcurrency(value)
			require.Error(t, err)
		})
	}
}

func TestHelmControllerConcurrency(t *testing.T) {
	helm, err := exec.LookPath("helm")
	if err != nil {
		t.Skip("helm is required for chart rendering tests")
	}
	render := func(t *testing.T, values string) ([]byte, error) {
		t.Helper()
		cmd := exec.Command(helm, "template", "test", "./deploy", "--show-only", "templates/deployment_manager.yaml", "--values", "-")
		cmd.Stdin = strings.NewReader(values)
		return cmd.CombinedOutput()
	}
	baseline, err := render(t, "{}")
	require.NoError(t, err, string(baseline))
	require.NotContains(t, string(baseline), "--controller-concurrency")
	for _, values := range []string{"manager: {concurrency: {}}", "manager: {concurrency: null}"} {
		output, err := render(t, values)
		require.NoError(t, err, string(output))
		require.Equal(t, string(baseline), string(output))
	}

	for _, values := range []string{
		"manager: {concurrency: {resource: 2, componentVersion: 4}}",
		"manager: {concurrency: {componentVersion: 2, resource: 3, snapshot: 4, localization: 5, configuration: 6, fluxDeployer: 7}}",
	} {
		output, err := render(t, values)
		require.NoError(t, err, string(output))
		var deployment appsv1.Deployment
		require.NoError(t, yaml.Unmarshal(output, &deployment))
		var overrides []string
		for _, arg := range deployment.Spec.Template.Spec.Containers[0].Args {
			if value, ok := strings.CutPrefix(arg, "--controller-concurrency="); ok {
				overrides = append(overrides, value)
			}
		}
		require.Len(t, overrides, 1)
		parsed, err := parseControllerConcurrency(overrides[0])
		require.NoError(t, err)
		if strings.Contains(values, "snapshot") {
			require.Len(t, parsed, 6)
		} else {
			require.Equal(t, "componentVersion=4,resource=2", overrides[0])
			require.Len(t, parsed, 2)
		}
	}

	for _, values := range []string{
		"manager: {concurrency: {unknown: 2}}",
		"manager: {concurrency: {resource: 0}}",
		"manager: {concurrency: {resource: -1}}",
		"manager: {concurrency: {resource: 1.5}}",
		"manager: {concurrency: {resource: true}}",
	} {
		output, err := render(t, values)
		require.Error(t, err, string(output))
	}
}
