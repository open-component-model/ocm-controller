// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/pkg/oci"
	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/accessmethods/localblob"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/accessmethods/ociartefact"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/accessmethods/ociblob"
	ocmapi "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/ocm.software/v3alpha1"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/cpi"
	ocmruntime "github.com/open-component-model/ocm/pkg/runtime"
)

func GetResource(snapshot v1alpha1.Snapshot, result interface{}) error {
	image := strings.TrimPrefix(snapshot.Status.Image, "http://")
	image = strings.TrimPrefix(image, "https://")
	repo, err := oci.NewRepository(image, oci.WithInsecure())
	if err != nil {
		return fmt.Errorf("failed to get repository: %w", err)
	}
	blob, err := repo.FetchBlob(snapshot.Spec.Digest)
	if err != nil {
		return fmt.Errorf("failed to fetch blob: %w", err)
	}
	content, err := io.ReadAll(blob)
	if err != nil {
		return fmt.Errorf("failed to read blob: %w", err)
	}
	return ocmruntime.DefaultYAMLEncoding.Unmarshal(content, result)
}

func GetImageReference(resource *ocmapi.Resource) (string, error) {
	accessSpec, err := GetResourceAccess(resource)
	if err != nil {
		return "", err
	}

	switch resource.Access.Type {
	case "localBlob":
		gs, err := accessSpec.(*localblob.AccessSpec).GlobalAccess.Evaluate(ocm.DefaultContext())
		if err != nil {
			return "", err
		}
		ref := gs.(*ociblob.AccessSpec).Reference
		sha := gs.(*ociblob.AccessSpec).Digest.String()
		return fmt.Sprintf("%s:%s@%s", ref, resource.GetVersion(), sha), nil
	case "ociblob":
		return accessSpec.(*ociblob.AccessSpec).Reference, nil
	case "ociArtefact":
		return accessSpec.(*ociartefact.AccessSpec).ImageReference, nil
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
	case "ociblob":
		accessSpec = &ociblob.AccessSpec{}
	case "ociArtefact":
		accessSpec = &ociartefact.AccessSpec{}
	}

	if err := ocmruntime.DefaultJSONEncoding.Unmarshal(rawAccessSpec, accessSpec); err != nil {
		return nil, fmt.Errorf("failed to unmarshal access spec: %w", err)
	}

	return accessSpec, err
}
