package event

import (
	eventv1 "github.com/fluxcd/pkg/apis/event/v1"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/events"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func New(recorder events.Recorder, obj conditions.Getter, related runtime.Object, metadata map[string]string, severity, msg string, args ...any) {
	if metadata == nil {
		metadata = map[string]string{}
	}

	reason := severity
	if r := conditions.GetReason(obj, meta.ReadyCondition); r != "" {
		reason = r
	}

	eventType := corev1.EventTypeNormal
	// events.k8s.io/v1 mandates an action; use the default Reconciled unless this is
	// an error. Revisit if call sites need finer-grained actions.
	action := eventv1.ActionReconciled
	if severity == eventv1.EventSeverityError {
		eventType = corev1.EventTypeWarning
		action = eventv1.ActionFailed
	}

	recorder.AnnotatedEventf(obj, related, metadata, eventType, reason, action, msg, args...)
}
