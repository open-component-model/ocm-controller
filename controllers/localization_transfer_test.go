package controllers

import (
	"io"
	"log"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
	"ocm.software/ocm/api/datacontext"
	"ocm.software/ocm/api/datacontext/attrs/rootcertsattr"
	"ocm.software/ocm/api/ocm"
	"ocm.software/ocm/api/ocm/compdesc"
	ocmmeta "ocm.software/ocm/api/ocm/compdesc/meta/v1"
	"ocm.software/ocm/api/ocm/extensions/accessmethods/ociartifact"
	"ocm.software/ocm/api/ocm/extensions/repositories/ctf"
	"ocm.software/ocm/api/ocm/extensions/repositories/ocireg"
	"ocm.software/ocm/api/ocm/ocmutils/localize"
	"ocm.software/ocm/api/ocm/tools/transfer"
	"ocm.software/ocm/api/ocm/tools/transfer/transferhandler/standard"
	"ocm.software/ocm/api/utils/accessio"
	ocmruntime "ocm.software/ocm/api/utils/runtime"
	"sigs.k8s.io/yaml"

	"github.com/open-component-model/ocm-controller/pkg/configdata"
)

// Exercise the same resource-copy path as `ocm transfer component --copy-resources`,
// then localize values and render a real Helm chart against the copied image.
// All registry traffic stays on loopback; no cluster or external images are needed.
func TestLocalizationAfterResourceTransfer(t *testing.T) {
	for _, digestOnly := range []bool{false, true} {
		testName := "tag"
		if digestOnly {
			testName = "digest"
		}
		t.Run(testName, func(t *testing.T) {
			sourceRegistry := httptest.NewTLSServer(registry.New(registry.Logger(log.New(io.Discard, "", 0))))
			t.Cleanup(sourceRegistry.Close)
			targetRegistry := httptest.NewTLSServer(registry.New(registry.Logger(log.New(io.Discard, "", 0))))
			t.Cleanup(targetRegistry.Close)
			octx := ocm.New()
			t.Cleanup(func() { _ = datacontext.Close(octx) })
			require.NoError(t, rootcertsattr.Get(octx).RegisterRootCertificates(sourceRegistry.Certificate()))
			require.NoError(t, rootcertsattr.Get(octx).RegisterRootCertificates(targetRegistry.Certificate()))

			image, err := random.Image(256, 1)
			require.NoError(t, err)
			digest, err := image.Digest()
			require.NoError(t, err)
			sourceTag, err := name.NewTag(strings.TrimPrefix(sourceRegistry.URL, "https://") + "/team/image:v1")
			require.NoError(t, err)
			require.NoError(t, remote.Write(sourceTag, image, remote.WithTransport(sourceRegistry.Client().Transport), remote.WithContext(t.Context())))
			sourceReference := sourceTag.Name()
			if digestOnly {
				sourceReference = sourceTag.Context().Name() + "@" + digest.String()
			}

			source, err := ctf.Create(octx, ctf.ACC_CREATE|ctf.ACC_WRITABLE, filepath.Join(t.TempDir(), "source"), 0o700, accessio.FormatDirectory)
			require.NoError(t, err)
			t.Cleanup(func() { _ = source.Close() })
			cv, err := source.NewComponentVersion("example.org/localization-demo", "1.0.0")
			require.NoError(t, err)
			t.Cleanup(func() { _ = cv.Close() })
			meta := compdesc.NewResourceMeta("image", "ociImage", ocmmeta.ExternalRelation)
			meta.Version = "1.0.0"
			require.NoError(t, cv.SetResource(meta, ociartifact.New(sourceReference)))
			target, err := ocireg.NewRepository(octx, strings.TrimPrefix(targetRegistry.URL, "https://")+"/components")
			require.NoError(t, err)
			t.Cleanup(func() { _ = target.Close() })
			require.NoError(t, transfer.TransferWithContext(t.Context(), cv, target, standard.ResourcesByValue(), standard.Recursive()))
			copied, err := target.LookupComponentVersion(cv.GetName(), cv.GetVersion())
			require.NoError(t, err)
			t.Cleanup(func() { _ = copied.Close() })

			property, separator := "tag", ":"
			if digestOnly {
				property, separator = "digest", "@"
			}
			configYAML := "localization:\n- file: values.yaml\n  fullyQualifiedRepository: image.repository\n  " + property + ": image." + property + "\n  resource:\n    name: image\n"
			var config configdata.ConfigData
			require.NoError(t, ocmruntime.DefaultYAMLEncoding.Unmarshal([]byte(configYAML), &config))
			resolved, err := resolveReference(config.Localization[0], nil, copied, octx)
			require.NoError(t, err)
			require.Equal(t, strings.TrimPrefix(targetRegistry.URL, "https://"), resolved.Context().RegistryStr())
			require.Equal(t, digest.String(), resolved.Identifier())
			if !digestOnly {
				require.Contains(t, resolved.String(), ":v1@"+digest.String(), "transfer must preserve the tag and add the digest")
			}

			var substitutions localize.Substitutions
			require.NoError(t, (&MutationReconcileLooper{}).performLocalization(octx, config.Localization[0], &substitutions, nil, copied))
			var mappings localize.ValueMappings
			for _, sub := range substitutions {
				mappings = append(mappings, sub.ValueMapping)
			}
			localized, err := localize.SubstituteMappingsForData(mappings, []byte("image:\n  repository: old\n  "+property+": old\n"))
			require.NoError(t, err)
			values := chartutil.Values{}
			require.NoError(t, yaml.Unmarshal(localized, &values))
			template := `apiVersion: v1
kind: Pod
spec:
  containers:
  - name: upgrade
    image: "{{ .Values.image.repository }}` + separator + `{{ .Values.image.` + property + ` }}"
`
			fixture := &chart.Chart{
				Metadata:  &chart.Metadata{Name: "localization-demo", Version: "1.0.0", APIVersion: "v2"},
				Templates: []*chart.File{{Name: "templates/pod.yaml", Data: []byte(template)}},
			}
			renderValues, err := chartutil.ToRenderValues(fixture, values, chartutil.ReleaseOptions{Name: "demo"}, nil)
			require.NoError(t, err)
			rendered, err := engine.Render(fixture, renderValues)
			require.NoError(t, err)
			var pod struct {
				Spec struct{ Containers []struct{ Image string } }
			}
			require.NoError(t, yaml.Unmarshal([]byte(rendered["localization-demo/templates/pod.yaml"]), &pod))
			require.Len(t, pod.Spec.Containers, 1)
			renderedImage := pod.Spec.Containers[0].Image
			parsed, err := name.ParseReference(renderedImage)
			require.NoError(t, err)
			require.Equal(t, resolved.Context().Name(), parsed.Context().Name())
			require.Equal(t, digest.String(), parsed.Identifier())
			copiedImage, err := remote.Image(parsed, remote.WithTransport(targetRegistry.Client().Transport), remote.WithContext(t.Context()))
			require.NoError(t, err)
			copiedDigest, err := copiedImage.Digest()
			require.NoError(t, err)
			require.Equal(t, digest, copiedDigest, "the rendered image must exist at the destination with the original digest")
			t.Logf("source: %s\ntransferred: %s\nlocalized values:\n%srendered image: %s", sourceReference, resolved.String(), localized, renderedImage)
		})
	}
}
