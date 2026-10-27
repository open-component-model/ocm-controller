package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/yaml"
)

func TestHelmIntervalJitterPercentage(t *testing.T) {
	for _, tt := range []struct {
		name  string
		value string
		want  string
	}{
		{name: "default", value: "{}", want: "15"},
		{name: "disabled", value: "manager: {intervalJitterPercentage: 0}", want: "0"},
		{name: "custom", value: "manager: {intervalJitterPercentage: 25}", want: "25"},
		{name: "largest", value: "manager: {intervalJitterPercentage: 99}", want: "99"},
		{name: "null uses CLI default", value: "manager: {intervalJitterPercentage: null}"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			chart, err := loader.Load("./deploy")
			require.NoError(t, err)
			overrides, err := chartutil.ReadValues([]byte(tt.value))
			require.NoError(t, err)
			values, err := chartutil.ToRenderValues(chart, overrides, chartutil.ReleaseOptions{
				Name:      "test",
				Namespace: "default",
				Revision:  1,
				IsInstall: true,
			}, nil)
			require.NoError(t, err)
			rendered, err := engine.Render(chart, values)
			require.NoError(t, err)
			templateName := chart.Name() + "/templates/deployment_manager.yaml"
			require.Contains(t, rendered, templateName)
			var deployment appsv1.Deployment
			require.NoError(t, yaml.Unmarshal([]byte(rendered[templateName]), &deployment))
			require.Len(t, deployment.Spec.Template.Spec.Containers, 1)
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
