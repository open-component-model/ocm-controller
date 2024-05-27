package status

import (
	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/open-component-model/ocm-controller/pkg/event"
	kuberecorder "k8s.io/client-go/tools/record"
)

// MarkNotReady sets the condition status of an Object to `Not Ready`.
func MarkNotReady(recorder kuberecorder.EventRecorder, obj conditions.Setter, reason, msg string) {
	conditions.Delete(obj, meta.ReconcilingCondition)
	conditions.MarkFalse(obj, meta.ReadyCondition, reason, msg)
	event.New(recorder, obj, nil, eventv1.EventSeverityError, msg)
}

// MarkAsStalled sets the condition status of an Object to `Stalled`.
func MarkAsStalled(recorder kuberecorder.EventRecorder, obj conditions.Setter, reason, msg string) {
	conditions.Delete(obj, meta.ReconcilingCondition)
	conditions.MarkFalse(obj, meta.ReadyCondition, reason, msg)
	conditions.MarkStalled(obj, reason, msg)
	event.New(recorder, obj, nil, eventv1.EventSeverityError, msg)
}

// MarkReady sets the condition status of an Object to `Ready`.
func MarkReady(recorder kuberecorder.EventRecorder, obj conditions.Setter, msg string, messageArgs ...any) {
	conditions.MarkTrue(obj, meta.ReadyCondition, meta.SucceededReason, msg, messageArgs...)
	conditions.Delete(obj, meta.ReconcilingCondition)
	event.New(recorder, obj, nil, eventv1.EventSeverityInfo, msg, messageArgs...)
}
