// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"errors"
	"fmt"

	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/accessmethods/localblob"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/accessmethods/ociartifact"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/accessmethods/ociblob"
	ocmapi "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/ocm.software/v3alpha1"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/cpi"
	ocmruntime "github.com/open-component-model/ocm/pkg/runtime"
)

// TODO: Replace this with something that can handle other access Types.
// Alternatively, extend it.
func GetImageReference(resource *ocmapi.Resource) (string, error) {
	accessSpec, err := GetResourceAccess(resource)
	if err != nil {
		return "", err
	}

	switch resource.Access.Type {
	case "localBlob":
		access, ok := accessSpec.(*localblob.AccessSpec)
		if !ok {
			return "", fmt.Errorf("access type was not localBlob: %+v", accessSpec)
		}

		if access.GlobalAccess == nil {
			return "", fmt.Errorf("access type doesn't have a global access method that is required for configuration/localization")
		}

		gs, err := access.GlobalAccess.Evaluate(ocm.DefaultContext())
		if err != nil {
			return "", err
		}
		ref := gs.(*ociblob.AccessSpec).Reference
		sha := gs.(*ociblob.AccessSpec).Digest.String()
		return fmt.Sprintf("%s:%s@%s", ref, resource.GetVersion(), sha), nil
	case "ociBlob":
		return accessSpec.(*ociblob.AccessSpec).Reference, nil
	case "ociArtifact":
		return accessSpec.(*ociartifact.AccessSpec).ImageReference, nil
	}

	return "", errors.New("could not get access information")
}

func GetResourceAccess(resource *ocmapi.Resource) (cpi.AccessSpec, error) {
	var accessSpec cpi.AccessSpec
	rawAccessSpec, err := resource.Access.GetRaw()
	if err != nil {
		return nil, err
	}

	switch resource.Access.Type {
	case "localBlob":
		accessSpec = &localblob.AccessSpec{}
	case "ociBlob":
		accessSpec = &ociblob.AccessSpec{}
	case "ociArtifact":
		accessSpec = &ociartifact.AccessSpec{}
	}

	if err := ocmruntime.DefaultJSONEncoding.Unmarshal(rawAccessSpec, accessSpec); err != nil {
		return nil, fmt.Errorf("failed to unmarshal access spec: %w", err)
	}

	return accessSpec, err
}
