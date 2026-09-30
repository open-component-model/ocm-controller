package status

import (
	eventv1 "github.com/fluxcd/pkg/apis/event/v1"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/events"
	"github.com/open-component-model/ocm-controller/pkg/event"
	"k8s.io/apimachinery/pkg/runtime"
)

// MarkNotReady sets the condition status of an Object to `Not Ready`.
func MarkNotReady(recorder events.Recorder, obj conditions.Setter, related runtime.Object, reason, msg string) {
	conditions.Delete(obj, meta.ReconcilingCondition)
	conditions.MarkFalse(obj, meta.ReadyCondition, reason, msg, []any{}...)
	event.New(recorder, obj, related, nil, eventv1.EventSeverityError, msg, []any{}...)
}

// MarkAsStalled sets the condition status of an Object to `Stalled`.
func MarkAsStalled(recorder events.Recorder, obj conditions.Setter, related runtime.Object, reason, msg string) {
	conditions.Delete(obj, meta.ReconcilingCondition)
	conditions.MarkFalse(obj, meta.ReadyCondition, reason, msg, []any{}...)
	conditions.MarkStalled(obj, reason, msg, []any{}...)
	event.New(recorder, obj, related, nil, eventv1.EventSeverityError, msg, []any{}...)
}

// MarkReady sets the condition status of an Object to `Ready`.
func MarkReady(recorder events.Recorder, obj conditions.Setter, related runtime.Object, msg string, messageArgs ...any) {
	conditions.MarkTrue(obj, meta.ReadyCondition, meta.SucceededReason, msg, messageArgs...)
	conditions.Delete(obj, meta.ReconcilingCondition)
	event.New(recorder, obj, related, nil, eventv1.EventSeverityInfo, msg, messageArgs...)
}
