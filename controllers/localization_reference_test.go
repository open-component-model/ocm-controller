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
