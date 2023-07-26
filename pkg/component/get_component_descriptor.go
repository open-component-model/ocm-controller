// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package component

import (
	"context"
	"fmt"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/open-component-model/ocm/v2/pkg/contexts/ocm/compdesc"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	ocmmetav1 "github.com/open-component-model/ocm/v2/pkg/contexts/ocm/compdesc/meta/v1"
)

func getComponentDescriptorObject(ctx context.Context, c client.Client, ref meta.NamespacedObjectReference) (*v1alpha1.ComponentDescriptor, error) {
	componentDescriptor := &v1alpha1.ComponentDescriptor{}
	if err := c.Get(ctx, types.NamespacedName{
		Name:      ref.Name,
		Namespace: ref.Namespace,
	}, componentDescriptor); err != nil {
		return nil, fmt.Errorf("failed to find component descriptor: %w", err)
	}
	return componentDescriptor, nil
}

func GetComponentDescriptor(ctx context.Context, c client.Client, refPath []ocmmetav1.Identity, obj v1alpha1.Reference) (*v1alpha1.ComponentDescriptor, error) {
	// Return early if there was no name defined.
	if len(refPath) == 0 {
		return getComponentDescriptorObject(ctx, c, obj.ComponentDescriptorRef)
	}

	// Handle the nested loop. If we get to this part, we check if the reference that we found
	// is the one we were looking for.
	//TODO: What about extra identity?
	if referencePathContainsName(obj.Name, refPath) {
		return getComponentDescriptorObject(ctx, c, obj.ComponentDescriptorRef)
	}

	// This is not the reference object we are looking for, let's dig deeper.
	for _, ref := range obj.References {
		desc, err := GetComponentDescriptor(ctx, c, refPath, ref)
		if err != nil {
			return nil, err
		}
		// recursive call for ref did not result in a reference
		// get the next ref, do the same lookup again
		if desc == nil {
			continue
		}

		return desc, nil
	}

	return nil, nil
}

func referencePathContainsName(name string, refPath []ocmmetav1.Identity) bool {
	for _, ref := range refPath {
		for k, v := range ref {
			if k == compdesc.SystemIdentityName && name == v {
				return true
			}
		}
	}
	return false
}
