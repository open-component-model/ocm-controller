// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"fmt"

	"github.com/fluxcd/pkg/runtime/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func patchObject(ctx context.Context, client client.Client, oldObject, newObject client.Object) error {
	patchHelper, err := patch.NewHelper(oldObject, client)
	if err != nil {
		return fmt.Errorf("failed to create patch helper: %w", err)
	}
	if err := patchHelper.Patch(ctx, newObject); err != nil {
		return fmt.Errorf("failed to patch object: %w", err)
	}
	return nil
}
