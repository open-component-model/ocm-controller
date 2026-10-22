package deploy_test

import (
	"bytes"
	"encoding/json"
	"io"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
)

func renderChart(t *testing.T, values map[string]interface{}) map[string]*unstructured.Unstructured {
	t.Helper()
	if _, err := exec.LookPath("helm"); err != nil {
		t.Skip("helm binary not found in PATH")
	}
	config, err := json.Marshal(values)
	require.NoError(t, err)
	command := exec.Command("helm", "template", "ocm-controller", ".", "--namespace", "ocm-system", "--values", "-")
	command.Stdin = bytes.NewReader(config)
	rendered, err := command.CombinedOutput()
	require.NoError(t, err, "%s", rendered)
	resources := make(map[string]*unstructured.Unstructured)
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(rendered), 4096)
	for {
		resource := &unstructured.Unstructured{}
		if err := decoder.Decode(resource); err == io.EOF {
			break
		} else {
			require.NoError(t, err)
		}
		if resource.GetKind() != "" {
			resources[resource.GetKind()+"/"+resource.GetName()] = resource
		}
	}
	return resources
}

// TestCertificateSecretReferences verifies that the cert-manager CA issuer
// references the bootstrap CA secret rather than the serving secret, so a
// separate bootstrap CA secret name can be used. The serving secret name is
// still bound to the volume mounts via a shared values anchor and is not
// separable here.
func TestCertificateSecretReferences(t *testing.T) {
	for _, test := range []struct {
		name, ca, serving string
		values            map[string]interface{}
	}{
		{"defaults", "ocm-registry-tls-certs", "ocm-registry-tls-certs", map[string]interface{}{"generateTlsCert": true}},
		{"separate ca secret", "custom-ca", "ocm-registry-tls-certs", map[string]interface{}{
			"generateTlsCert": true,
			"bootstrap":       map[string]interface{}{"secretName": "custom-ca"},
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			resources := renderChart(t, map[string]interface{}{"tlsCert": test.values})
			for _, reference := range []struct {
				resource, want string
				path           []string
			}{
				{"Certificate/ocm-bootstrap-certificate", test.ca, []string{"spec", "secretName"}},
				{"ClusterIssuer/ocm-certificate-issuer", test.ca, []string{"spec", "ca", "secretName"}},
				{"Certificate/ocm-registry-tls-certs", test.serving, []string{"spec", "secretName"}},
			} {
				resource := resources[reference.resource]
				require.NotNil(t, resource)
				value, found, err := unstructured.NestedString(resource.Object, reference.path...)
				require.NoError(t, err)
				require.True(t, found)
				require.Equal(t, reference.want, value)
			}
		})
	}
}
