// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package status

import (
	"github.com/fluxcd/pkg/runtime/conditions"
)

// IdentifiableClientObject defines an object which can create an identity for itself.
type IdentifiableClientObject interface {
	Mutator
	conditions.Setter

	// GetVID constructs an identifier for an object.
	GetVID() map[string]string
}

// Mutator allows mutating specific status fields of an object.
type Mutator interface {
	// SetObservedGeneration mutates the observed generation field of an object.
	SetObservedGeneration(v int64)
}
