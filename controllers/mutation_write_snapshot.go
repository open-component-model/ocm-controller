// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"fmt"
	"os"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/pkg/snapshot"
	ocmmetav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (m *MutationReconcileLooper) writeSnapshot(ctx context.Context, obj v1alpha1.MutationObject, sourceDir string, identity ocmmetav1.Identity) error {
	artifactPath, err := os.CreateTemp("", "snapshot-artifact-*.tgz")
	if err != nil {
		return fmt.Errorf("fs error: %w", err)
	}
	defer os.Remove(artifactPath.Name())

	if err := buildTar(artifactPath.Name(), sourceDir); err != nil {
		return fmt.Errorf("build tar error: %w", err)
	}

	snapshotDigest, err := m.writeToCache(ctx, identity, artifactPath.Name(), obj.GetResourceVersion())
	if err != nil {
		return err
	}

	snapshotName, err := snapshot.GetSnapshotNameForObject(obj.GetName(), obj)
	if err != nil {
		return fmt.Errorf("failed to get snapshotname: %w", err)
	}

	snapshotCR := &v1alpha1.Snapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      snapshotName,
			Namespace: obj.GetNamespace(),
		},
	}

	_, err = controllerutil.CreateOrUpdate(ctx, m.Client, snapshotCR, func() error {
		if snapshotCR.ObjectMeta.CreationTimestamp.IsZero() {
			if err := controllerutil.SetOwnerReference(obj, snapshotCR, m.Scheme); err != nil {
				return fmt.Errorf("failed to set owner reference on snapshot: %w", err)
			}
		}
		snapshotCR.Spec = v1alpha1.SnapshotSpec{
			Identity: identity,
			Digest:   snapshotDigest,
			Tag:      obj.GetResourceVersion(),
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to create or update snapshot: %w", err)
	}

	obj.GetStatus().LatestSnapshotDigest = snapshotDigest
	obj.GetStatus().SnapshotName = snapshotName

	return nil
}
