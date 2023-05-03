// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type SnapshotDigestChangedPredicate struct {
	predicate.Funcs
}

func (SnapshotDigestChangedPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}

	oldSnapshot, ok := e.ObjectOld.(*v1alpha1.Snapshot)
	if !ok {
		return false
	}

	newSnapshot, ok := e.ObjectNew.(*v1alpha1.Snapshot)
	if !ok {
		return false
	}

	if oldSnapshot.GetDigest() == "" && newSnapshot.GetDigest() != "" {
		return true
	}

	if oldSnapshot.GetDigest() != "" && newSnapshot.GetDigest() != "" &&
		(oldSnapshot.GetDigest() != newSnapshot.GetDigest()) {
		return true
	}

	return false
}
