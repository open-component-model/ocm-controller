package controllers

import (
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
)

type ComponentVersionChangedPredicate struct {
	predicate.Funcs
}

func (ComponentVersionChangedPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}

	oldComponentVersion, ok := e.ObjectOld.(*v1alpha1.ComponentVersion)
	if !ok {
		return false
	}

	newComponentVersion, ok := e.ObjectNew.(*v1alpha1.ComponentVersion)
	if !ok {
		return false
	}

	if oldComponentVersion.GetVersion() == "" && newComponentVersion.GetVersion() != "" {
		return true
	}

	if oldComponentVersion.GetVersion() != "" && newComponentVersion.GetVersion() != "" &&
		(oldComponentVersion.GetVersion() != newComponentVersion.GetVersion()) {
		return true
	}

	return false
}
