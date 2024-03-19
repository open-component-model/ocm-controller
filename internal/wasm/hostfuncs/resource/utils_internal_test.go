package resource

import (
	"encoding/json"
	"testing"

	ocmcontext "github.com/open-component-model/ocm-controller/pkg/fakes"
	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	ocmmetav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/repositories/genericocireg"
	"github.com/stretchr/testify/require"
)

func NewTestComponentWithData(t *testing.T, data []byte) (ocm.ComponentVersionAccess, error) {
	componentName := "ocm.software/test"
	componentVersion := "v1.0.0"
	resourceName := "data"
	resourceVersion := "v1.0.0"

	octx := ocmcontext.NewFakeOCMContext()

	comp := &ocmcontext.Component{
		Name:    componentName,
		Version: componentVersion,
	}
	res := &ocmcontext.Resource[*ocm.ResourceMeta]{
		Name:    resourceName,
		Version: resourceVersion,
		Labels: ocmmetav1.Labels{
			{
				Name:  "test",
				Value: json.RawMessage(`"data"`),
			},
		},
		Data:      data,
		Component: comp,
		Kind:      "localBlob",
		Type:      "ociBlob",
	}

	comp.Resources = append(comp.Resources, res)

	octx.AddComponent(comp)

	repo, err := octx.RepositoryForSpec(&genericocireg.RepositorySpec{})
	require.NoError(t, err)

	return repo.LookupComponentVersion(componentName, componentVersion)
}
