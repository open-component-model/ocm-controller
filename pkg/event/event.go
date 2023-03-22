// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package event

import (
	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"

	corev1 "k8s.io/api/core/v1"
	kuberecorder "k8s.io/client-go/tools/record"
)

func New(recorder kuberecorder.EventRecorder, obj conditions.Getter, severity, msg string, metadata map[string]string) {
	if metadata == nil {
		metadata = map[string]string{}
	}

	reason := severity
	if r := conditions.GetReason(obj, meta.ReadyCondition); r != "" {
		reason = r
	}

	eventType := corev1.EventTypeNormal
	if severity == eventv1.EventSeverityError {
		eventType = corev1.EventTypeWarning
	}

	recorder.AnnotatedEventf(obj, metadata, eventType, reason, msg)
}
