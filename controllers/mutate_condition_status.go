package controllers

import (
	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/open-component-model/ocm-controller/pkg/event"
	kuberecorder "k8s.io/client-go/tools/record"
)

// MarkAsFailed sets the condition and progressive status of an Object to the set reason, msg, and format for the
// progressive status.
func MarkAsFailed(recorder kuberecorder.EventRecorder, obj conditions.Setter, reason, msg string) {
	conditions.MarkFalse(obj, meta.ReadyCondition, reason, msg)
	event.New(recorder, obj, eventv1.EventSeverityError, msg, nil)
}
