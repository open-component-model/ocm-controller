package controllers

import (
	"errors"
	"path/filepath"
	"testing"

	"ocm.software/ocm/api/datacontext"
	ocm "ocm.software/ocm/api/ocm"
	"ocm.software/ocm/api/ocm/compdesc"
	ocmmeta "ocm.software/ocm/api/ocm/compdesc/meta/v1"
	"ocm.software/ocm/api/ocm/extensions/accessmethods/ociartifact"
	"ocm.software/ocm/api/ocm/extensions/repositories/ctf"
	"ocm.software/ocm/api/utils/accessio"

	"github.com/open-component-model/ocm-controller/pkg/configdata"
)

type localizationLifecycleComponent struct {
	ocm.ComponentVersionAccess
	resource ocm.ResourceAccess
	resolved *localizationLifecycleComponent
	closed   int
}

// Use a real local OCM repository to verify the reference count, rather than
// relying on the controller's fakes (whose Close methods are no-ops).
func TestLocalizationPreservesComponentReferenceCount(t *testing.T) {
	octx := ocm.New()
	t.Cleanup(func() { _ = datacontext.Close(octx) })
	repo, err := ctf.Create(octx, ctf.ACC_CREATE|ctf.ACC_WRITABLE, filepath.Join(t.TempDir(), "ctf"), 0o700, accessio.FormatDirectory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	cv, err := repo.NewComponentVersion("example.org/component", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cv.Close() })
	if err := cv.SetResource(compdesc.NewResourceMeta("image", "ociImage", ocmmeta.ExternalRelation), ociartifact.New("example.org/images/app@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")); err != nil {
		t.Fatal(err)
	}
	counter, ok := cv.(interface{ RefCount() int })
	if !ok {
		t.Fatal("component view does not expose its reference count")
	}
	before := counter.RefCount()
	rule := configdata.LocalizationRule{}
	rule.Resource.Name = "image"
	for i := 0; i < 100; i++ {
		if _, err := resolveReference(rule, nil, cv, octx); err != nil {
			t.Fatal(err)
		}
	}
	if after := counter.RefCount(); after != before {
		t.Fatalf("100 localization lookups retained %d extra OCM references (before=%d, after=%d)", after-before, before, after)
	}
}

func (c *localizationLifecycleComponent) Dup() (ocm.ComponentVersionAccess, error) {
	return c.resolved, nil
}
func (c *localizationLifecycleComponent) Repository() ocm.Repository { return nil }
func (c *localizationLifecycleComponent) GetResource(ocmmeta.Identity) (ocm.ResourceAccess, error) {
	return c.resource, nil
}
func (c *localizationLifecycleComponent) Close() error { c.closed++; return nil }

type localizationLifecycleResource struct {
	ocm.ResourceAccess
	accessErr error
}

func (r *localizationLifecycleResource) Access() (ocm.AccessSpec, error) {
	if r.accessErr != nil {
		return nil, r.accessErr
	}
	return &ociartifact.AccessSpec{ImageReference: "example.org/images/app:1.0.0"}, nil
}

func TestLocalizationReleasesResolvedComponent(t *testing.T) {
	for _, accessFails := range []bool{false, true} {
		name := "success"
		if accessFails {
			name = "access failure"
		}
		t.Run(name, func(t *testing.T) {
			resource := &localizationLifecycleResource{}
			if accessFails {
				resource.accessErr = errors.New("access failed")
			}
			resolved := &localizationLifecycleComponent{resource: resource}
			root := &localizationLifecycleComponent{resolved: resolved}
			rule := configdata.LocalizationRule{}
			rule.Resource.Name = "image"
			_, err := resolveReference(rule, nil, root, nil)
			if (err != nil) != accessFails {
				t.Fatalf("unexpected resolve result: %v", err)
			}
			if resolved.closed != 1 {
				t.Errorf("resolved component closed %d times, want 1", resolved.closed)
			}
			if root.closed != 0 {
				t.Fatal("resolver closed caller's original component handle")
			}
		})
	}
}
