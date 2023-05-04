// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/ocm.software/v3alpha1"
	ocmruntime "github.com/open-component-model/ocm/pkg/runtime"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/pkg/component"
	"github.com/open-component-model/ocm-controller/pkg/snapshot"
)

type componentGenerator struct {
	name       string
	version    string
	resources  []resource
	references []reference
}

type resource struct {
	name    string
	version string
	image   string
}

type reference struct {
	name      string
	version   string
	component string
}

func TestPopulateReferences(t *testing.T) {
	ctx := context.Background()
	cueCtx := cuecontext.New()

	frontend := (&componentGenerator{
		name:    "frontend",
		version: "v1.0.0",
		resources: []resource{{
			name:    "web-server",
			version: "1.23.3-alpine",
			image:   "nginx:1.23-3-alpine",
		}},
		references: []reference{{
			name:      "backend",
			version:   "v2.0.3",
			component: "backend",
		}},
	}).build(true)
	assert.NotNil(t, frontend)

	backend := (&componentGenerator{
		name:    "backend",
		version: "v2.0.3",
		resources: []resource{{
			name:    "api",
			version: "v1.0.2",
			image:   "api:v1.0.2",
		}},
		references: []reference{
			{
				name:      "cache",
				version:   "v6.0.0",
				component: "redis",
			},
			{
				name:      "database",
				version:   "v12.0.2",
				component: "postgres",
			},
		},
	}).build(false)
	assert.NotNil(t, backend)

	cache := (&componentGenerator{
		name:    "redis",
		version: "v6.0.0",
		resources: []resource{{
			name:    "redis",
			version: "redis:6.3.9",
			image:   "redis:6.3.9",
		}},
	}).build(false)
	assert.NotNil(t, cache)

	database := (&componentGenerator{
		name:    "postgres",
		version: "v12.0.2",
		resources: []resource{{
			name:    "postgres",
			version: "12.0.2",
			image:   "postgres:12.0.2",
		}},
	}).build(false)
	assert.NotNil(t, database)

	objs := []client.Object{frontend, backend, cache, database}

	client := env.FakeKubeClient(WithObjects(objs...))

	ociWriter := snapshot.NewOCIWriter(client, nil, env.scheme)
	m := &MutationReconcileLooper{
		Scheme:         env.scheme,
		Client:         client,
		SnapshotWriter: ociWriter,
	}

	root := cueCtx.CompileString("component:{}").FillPath(cue.ParsePath("component"), cueCtx.Encode(frontend.Spec))

	result, err := m.populateReferences(ctx, root, frontend.GetNamespace())
	require.NoError(t, err)

	rootRes := result.LookupPath(cue.ParsePath("component.resources"))
	rootResLen, err := rootRes.Len().Int64()
	require.NoError(t, err)
	assert.Equal(t, 1, int(rootResLen), "frontend component has a single resource")
	rootName, err := rootRes.LookupPath(cue.MakePath(cue.Index(0), cue.Str("name"))).String()
	require.NoError(t, err)
	assert.Equal(t, "web-server", rootName, "the frontend resource is named 'web-server'")

	rootRef := result.LookupPath(cue.ParsePath("component.references"))
	rootRefLen, err := rootRef.Len().Int64()
	require.NoError(t, err)
	assert.Equal(t, 1, int(rootRefLen), "frontend component has a single reference")

	rootRefIter, err := rootRef.List()
	require.NoError(t, err)
	for rootRefIter.Next() {
		v := rootRefIter.Value()
		res := v.LookupPath(cue.ParsePath("component.resources"))
		resLen, err := res.Len().Int64()
		require.NoError(t, err)
		assert.Equal(t, 1, int(resLen), "backend component has a single resource")
		resName, err := v.LookupPath(cue.ParsePath("component.resources[0].name")).String()
		require.NoError(t, err)
		assert.Equal(t, "api", resName, "the backend resource is named 'api'")

		refs := v.LookupPath(cue.ParsePath("component.references"))
		refLen, err := refs.Len().Int64()
		require.NoError(t, err)
		assert.Equal(t, 2, int(refLen), "backend component has two references")

		refIter, err := rootRef.List()
		require.NoError(t, err)
		for refIter.Next() {
			rv := refIter.Value()
			rvRes := rv.LookupPath(cue.ParsePath("component.resources"))
			rvResLen, err := rvRes.Len().Int64()
			require.NoError(t, err)
			assert.Equal(t, 1, int(rvResLen), "backend references each have a single resource")
		}
	}
}

func (c *componentGenerator) build(isRoot bool) *v1alpha1.ComponentDescriptor {
	resources := make([]v3alpha1.Resource, len(c.resources))
	for i, r := range c.resources {
		resources[i] = makeImageResource(r.name, r.version, r.image)
	}
	references := make([]v3alpha1.Reference, len(c.references))
	for i, r := range c.references {
		references[i] = makeReference(r.name, r.version, r.component)
	}

	if !isRoot {
		var err error
		c.name, err = component.ConstructUniqueName(c.name, c.version, v1.Identity{})
		if err != nil {
			return nil
		}
	}

	return makeComponentDescriptor(c.name, c.version, resources, references)
}

func makeComponentDescriptor(name, version string, resources []v3alpha1.Resource, references []v3alpha1.Reference) *v1alpha1.ComponentDescriptor {
	namespace := "default"
	return &v1alpha1.ComponentDescriptor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.ComponentDescriptorSpec{
			Version: version,
			ComponentVersionSpec: v3alpha1.ComponentVersionSpec{
				Resources:  resources,
				References: references,
			},
		},
		Status: v1alpha1.ComponentDescriptorStatus{},
	}
}

func makeImageResource(name, version, image string) v3alpha1.Resource {
	return v3alpha1.Resource{
		ElementMeta: v3alpha1.ElementMeta{
			Name:    name,
			Version: version,
		},
		Type:     "ociImage",
		Relation: "external",
		Access: &ocmruntime.UnstructuredTypedObject{
			Object: map[string]interface{}{
				"type":           "ociArtifact",
				"imageReference": image,
			},
		},
	}
}

func makeReference(name, version, component string) v3alpha1.Reference {
	return v3alpha1.Reference{
		ElementMeta: v3alpha1.ElementMeta{
			Name:    name,
			Version: version,
		},
		ComponentName: component,
	}
}
