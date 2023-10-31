package controllers

import (
	"github.com/fluxcd/pkg/runtime/conditions"
)

// IdentifiableClientObject defines an object which can create an identity for itself.
type IdentifiableClientObject interface {
	StatusMutator
	conditions.Setter

	// GetVID constructs an identifier for an object.
	GetVID() map[string]string
}

// StatusMutator allows mutating specific status fields of an object.
type StatusMutator interface {
	// SetObservedGeneration mutates the observed generation field of an object.
	SetObservedGeneration(v int64)
}
