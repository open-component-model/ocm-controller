// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package status

import (
	"context"
	"time"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/patch"
	"github.com/open-component-model/ocm-controller/pkg/event"
	kuberecorder "k8s.io/client-go/tools/record"
)

// UpdateStatus takes an object which can identify itself and updates its status including ObservedGeneration.
func UpdateStatus(
	ctx context.Context,
	patchHelper *patch.SerialPatcher,
	obj IdentifiableClientObject,
	recorder kuberecorder.EventRecorder,
	requeue time.Duration,
	err error,
) error {
	// If still reconciling then reconciliation did not succeed, set to ProgressingWithRetry to
	// indicate that reconciliation will be retried.
	// This will add another indicator that we are indeed doing something. This is in addition to
	// the status that is already present on the object which is the Ready condition.
	if conditions.IsReconciling(obj) && err != nil {
		reconciling := conditions.Get(obj, meta.ReconcilingCondition)
		reconciling.Reason = meta.ProgressingWithRetryReason
		conditions.Set(obj, reconciling)
		event.New(recorder, obj, obj.GetVID(), eventv1.EventSeverityError, "Reconciliation did not succeed, retrying in %s", requeue)
	}

	// Set status observed generation option if the component is ready.
	if conditions.IsReady(obj) {
		obj.SetObservedGeneration(obj.GetGeneration())
		event.New(recorder, obj, obj.GetVID(), eventv1.EventSeverityInfo, "Reconciliation finished, next run in %s", requeue)
	}

	// Update the object.
	return patchHelper.Patch(ctx, obj)
}
