// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	mh "github.com/open-component-model/pkg/metrics"
)

const (
	metricsComponent = "ocm_controller"
)

func init() {
	metrics.Registry.MustRegister(
		ComponentVersionReconciledTotal,
		ComponentVersionReconcileFailed,
		ConfigurationReconcileFailed,
		LocalizationReconcileFailed,
		ResourceReconcileFailed,
		SnapshotNumberOfBytesReconciled,
		MPASProductReconciledStatus,
	)
}

// ComponentVersionReconciledTotal counts the number times a component version was reconciled.
var ComponentVersionReconciledTotal = mh.MustRegisterCounterVec(
	"ocm_system",
	metricsComponent,
	"component_version_reconciled_total",
	"Number of times a component version was reconciled",
	"component", "version",
)

// ComponentVersionReconcileFailed counts the number times we failed to reconcile a component version.
var ComponentVersionReconcileFailed = mh.MustRegisterCounterVec(
	"ocm_system",
	metricsComponent,
	"component_version_reconcile_failed",
	"Number of times a component version failed to reconcile",
	"component",
)

// ConfigurationReconcileFailed counts the number times we failed to reconcile a Configuration.
var ConfigurationReconcileFailed = mh.MustRegisterCounterVec(
	"ocm_system",
	metricsComponent,
	"configuration_reconcile_failed",
	"Number of times a configuration failed to reconcile",
	"configuration",
)

// LocalizationReconcileFailed counts the number times we failed to reconcile a Localization.
var LocalizationReconcileFailed = mh.MustRegisterCounterVec(
	"ocm_system",
	metricsComponent,
	"localization_reconcile_failed",
	"Number of times a localization failed to reconcile",
	"localization",
)

// ResourceReconcileFailed counts the number times we failed to reconcile a resource.
var ResourceReconcileFailed = mh.MustRegisterCounterVec(
	"ocm_system",
	metricsComponent,
	"resource_reconcile_failed",
	"Number of times a resource failed to reconcile",
	"resource",
)

// SnapshotNumberOfBytesReconciled number of bytes reconciled through snapshots.
var SnapshotNumberOfBytesReconciled = mh.MustRegisterGaugeVec(
	"ocm_system",
	metricsComponent,
	"snapshot_number_of_bytes_reconciled",
	"Number of bytes reconciled by a snapshot",
	"snapshot", "sha",
)

// MPASProductReconciledStatus updates the status of an MPAS product component.
var MPASProductReconciledStatus = mh.MustRegisterCounterVec(
	"mpas_system",
	metricsComponent,
	mh.MPASProductInstallationCounterLabel,
	"The status of an mpas product.",
	"product", "status",
)
