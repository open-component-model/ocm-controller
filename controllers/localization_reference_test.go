package controllers

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/stretchr/testify/require"
	"ocm.software/ocm/api/datacontext"
	"ocm.software/ocm/api/ocm"
	"ocm.software/ocm/api/ocm/ocmutils/localize"
	ocmruntime "ocm.software/ocm/api/utils/runtime"
	"sigs.k8s.io/yaml"

	"github.com/open-component-model/ocm-controller/pkg/configdata"
)

func TestLocalizationPreservesTagAndDigest(t *testing.T) {
	const repository = "registry.example:5000/team/image"
	digest := "sha256:" + strings.Repeat("a", 64)
	for _, tt := range []struct {
		name, reference, wantTag string
	}{
		{"tag", repository + ":v1", "v1"},
		{"tag and digest", repository + ":v1@" + digest, "v1@" + digest},
		{"digest only", repository + "@" + digest, digest},
		{"implicit latest", repository, "latest"},
		{"short pinned name", "alpine:v1@" + digest, "v1@" + digest},
	} {
		t.Run(tt.name, func(t *testing.T) {
			octx := ocm.New()
			t.Cleanup(func() { _ = datacontext.Close(octx) })
			component := getMockComponent(DefaultComponent, func(access map[string]any) {
				access["type"] = "ociArtifact"
				access["imageReference"] = tt.reference
			})
			rule := configdata.LocalizationRule{
				File: "values.yaml", Resource: configdata.ResourceItem{Name: "introspect-image"},
				FullyQualifiedRepository: "repository", Tag: "tag", Image: "image",
			}
			var subs localize.Substitutions
			require.NoError(t, (&MutationReconcileLooper{}).performLocalization(octx, rule, &subs, nil, component))
			mappings := make(localize.ValueMappings, 0, len(subs))
			for _, sub := range subs {
				mappings = append(mappings, sub.ValueMapping)
			}
			data, err := localize.SubstituteMappingsForData(mappings, []byte("repository: old\ntag: old\nimage: old\n"))
			require.NoError(t, err)
			var values struct{ Repository, Tag, Image string }
			require.NoError(t, yaml.Unmarshal(data, &values))
			require.Equal(t, tt.wantTag, values.Tag)

			// Full-image and repository localization retain their existing output.
			original, err := name.ParseReference(tt.reference)
			require.NoError(t, err)
			require.Equal(t, original.Context().Name(), values.Repository)
			require.Equal(t, original.Name(), values.Image)
			if strings.Contains(tt.wantTag, "@") {
				// This is the image expression used by repository:tag charts.
				rendered := fmt.Sprintf("%s:%s", values.Repository, values.Tag)
				ref, err := name.ParseReference(rendered)
				require.NoError(t, err)
				require.Equal(t, digest, ref.Identifier(), "the chart must retain the digest pin")
			}
		})
	}
}

func TestLocalizationDigest(t *testing.T) {
	const repository = "registry.example:5000/team/image"
	digest := "sha256:" + strings.Repeat("a", 64)
	sha512 := "sha512:" + strings.Repeat("b", 128)
	artifact := func(ref string) map[string]any {
		return map[string]any{"type": "ociArtifact", "imageReference": ref}
	}
	blob := map[string]any{"type": "ociBlob", "ref": repository, "digest": digest, "mediaType": "application/vnd.oci.image.manifest.v1+json", "size": 1}
	for _, tt := range []struct {
		name, wantDigest, wantError string
		access                      map[string]any
	}{
		{name: "tag and digest", access: artifact(repository + ":v1@" + digest), wantDigest: digest},
		{name: "digest only", access: artifact(repository + "@" + digest), wantDigest: digest},
		{name: "sha512", access: artifact(repository + "@" + sha512), wantDigest: sha512},
		{name: "OCI blob", access: blob, wantDigest: digest},
		{name: "local blob global blob", access: map[string]any{"type": "localBlob", "localReference": "local", "globalAccess": blob}, wantDigest: digest},
		{name: "local blob global artifact", access: map[string]any{"type": "localBlob", "localReference": "local", "globalAccess": artifact(repository + ":v1@" + digest)}, wantDigest: digest},
		{name: "tag only", access: artifact(repository + ":v1"), wantError: "no digest is present"},
		{name: "implicit latest", access: artifact(repository), wantError: "no digest is present"},
		{name: "malformed reference", access: artifact("invalid:@:v1"), wantError: "failed to parse access reference"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			octx := ocm.New()
			t.Cleanup(func() { _ = datacontext.Close(octx) })
			component := getMockComponent(DefaultComponent, func(access map[string]any) {
				clear(access)
				for k, v := range tt.access {
					access[k] = v
				}
			})
			var config configdata.ConfigData
			require.NoError(t, ocmruntime.DefaultYAMLEncoding.Unmarshal([]byte(`localization:
- file: values.yaml
  fullyQualifiedRepository: repository
  digest: digest
  tag: tag
  resource:
    name: introspect-image
`), &config))
			var subs localize.Substitutions
			err := (&MutationReconcileLooper{}).performLocalization(octx, config.Localization[0], &subs, nil, component)
			if tt.wantError != "" {
				require.ErrorContains(t, err, tt.wantError)
				return
			}
			require.NoError(t, err)
			var mappings localize.ValueMappings
			for _, sub := range subs {
				mappings = append(mappings, sub.ValueMapping)
			}
			data, err := localize.SubstituteMappingsForData(mappings, []byte("repository: old\ndigest: old\ntag: old\n"))
			require.NoError(t, err)
			var values struct{ Repository, Digest, Tag string }
			require.NoError(t, yaml.Unmarshal(data, &values))
			require.Equal(t, tt.wantDigest, values.Digest)
			rendered, err := name.ParseReference(values.Repository + "@" + values.Digest)
			require.NoError(t, err)
			require.Equal(t, tt.wantDigest, rendered.Identifier())
			if !strings.Contains(values.Tag, "@") {
				require.Equal(t, tt.wantDigest, values.Tag, "legacy digest-only tag output must remain unchanged")
			}
		})
	}
}
