// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"fmt"
	"os"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (m *MutationReconcileLooper) writeSnapshot(
	ctx context.Context,
	template v1alpha1.SnapshotTemplateSpec,
	owner client.Object,
	sourceDir string,
	identity v1alpha1.Identity,
	resourceType string,
) (string, error) {
	artifactPath, err := os.CreateTemp("", "snapshot-artifact-*.tgz")
	if err != nil {
		return "", fmt.Errorf("fs error: %w", err)
	}
	defer os.Remove(artifactPath.Name())

	if err := buildTar(artifactPath.Name(), sourceDir); err != nil {
		return "", fmt.Errorf("build tar error: %w", err)
	}

	snapshotDigest, err := m.writeToCache(ctx, identity, artifactPath.Name(), owner.GetResourceVersion())
	if err != nil {
		return "", err
	}

	snapshotCR := &v1alpha1.Snapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      template.Name,
			Namespace: owner.GetNamespace(),
		},
	}

	snapshotCR.SetContentType(resourceType)

	_, err = controllerutil.CreateOrUpdate(ctx, m.Client, snapshotCR, func() error {
		if snapshotCR.ObjectMeta.CreationTimestamp.IsZero() {
			if err := controllerutil.SetOwnerReference(owner, snapshotCR, m.Scheme); err != nil {
				return fmt.Errorf("failed to set owner reference on snapshot: %w", err)
			}
		}
		snapshotCR.Spec = v1alpha1.SnapshotSpec{
			Identity:         identity,
			CreateFluxSource: template.CreateFluxSource,
			Digest:           snapshotDigest,
			Tag:              owner.GetResourceVersion(),
		}

		if template.Tag != "" {
			snapshotCR.Spec.DuplicateTagToTag = template.Tag
		}

		return nil
	})

	if err != nil {
		return "", fmt.Errorf("failed to create or update component descriptor: %w", err)
	}

	return snapshotDigest, nil
}
