package controllers

import (
	"context"
	"fmt"
	"time"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/patch"
	"github.com/open-component-model/ocm-controller/pkg/event"
	kuberecorder "k8s.io/client-go/tools/record"
)

// DeferredStatusUpdate takes an object which can identify itself and updates its status including ObservedGeneration.
func DeferredStatusUpdate(
	ctx context.Context,
	patchHelper *patch.SerialPatcher,
	obj IdentifiableClientObject,
	recorder kuberecorder.EventRecorder,
	requeue time.Duration,
) error {
	// If still reconciling then reconciliation did not succeed, set to ProgressingWithRetry to
	// indicate that reconciliation will be retried.
	if conditions.IsReconciling(obj) {
		reconciling := conditions.Get(obj, meta.ReconcilingCondition)
		reconciling.Reason = meta.ProgressingWithRetryReason
		conditions.Set(obj, reconciling)
		msg := fmt.Sprintf("Reconciliation did not succeed, retrying in %s", requeue)
		event.New(recorder, obj, eventv1.EventSeverityError, msg, obj.GetVID())
	}

	// Set status observed generation option if the component is ready.
	if conditions.IsReady(obj) {
		obj.SetObservedGeneration(obj.GetGeneration())
		msg := fmt.Sprintf("Reconciliation finished, next run in %s", requeue)
		event.New(recorder, obj, eventv1.EventSeverityInfo, msg, obj.GetVID())
	}

	// Update the object.
	return patchHelper.Patch(ctx, obj)
}
