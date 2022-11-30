// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/accessmethods/localblob"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/accessmethods/ociartefact"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/accessmethods/ociblob"
	ocmapi "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/ocm.software/v3alpha1"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/cpi"
	ocmruntime "github.com/open-component-model/ocm/pkg/runtime"
)

func GetResource(ctx context.Context, snapshot v1alpha1.Snapshot, version, ociRegistryAddress string, result interface{}) error {
	//ref = strings.TrimPrefix("http://", ref)
	digest, err := name.NewDigest(fmt.Sprintf("%s@%s", snapshot.Spec.Ref, snapshot.Spec.Digest), name.Insecure)
	if err != nil {
		return fmt.Errorf("failed to create digest: %w", err)
	}

	// proxy image requests via the in-cluster oci-registry
	proxyURL, err := url.Parse(fmt.Sprintf("http://%s", ociRegistryAddress))
	if err != nil {
		return fmt.Errorf("failed to parse oci registry url: %w", err)
	}

	// create transport to the in-cluster oci-registry
	tr := newCustomTransport(remote.DefaultTransport.(*http.Transport).Clone(), proxyURL)

	// set context values to be transmitted as headers on the registry requests
	for k, v := range map[string]string{
		"registry":   digest.Repository.Registry.String(),
		"repository": digest.Repository.String(),
		"digest":     digest.String(),
		"image":      digest.Name(),
		"tag":        version,
	} {
		ctx = context.WithValue(ctx, contextKey(k), v)
	}

	// fetch the layer
	remoteLayer, err := remote.Layer(digest, remote.WithTransport(tr), remote.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("failed to retrieve layer: %w", err)
	}

	data, err := remoteLayer.Uncompressed()
	if err != nil {
		return fmt.Errorf("failed to read layer: %w", err)
	}

	configBytes, err := io.ReadAll(data)
	if err != nil {
		return fmt.Errorf("failed to read layer: %w", err)
	}

	return ocmruntime.DefaultYAMLEncoding.Unmarshal(configBytes, result)
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
