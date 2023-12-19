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
		ConfigurationReconcileSuccess,
		LocalizationReconcileFailed,
		LocalizationReconcileSuccess,
		ResourceReconcileFailed,
		ResourceReconcileSuccess,
		FluxDeployerReconcileFailed,
		FluxDeployerReconcileSuccess,
		SnapshotNumberOfBytesReconciled,
		SnapshotReconcileSuccess,
		SnapshotReconcileFailed,
		MPASComponentVersionReconciledStatus,
		MPASLocationReconciledStatus,
		MPASConfigurationReconciledStatus,
		MPASDeployerReconciledStatus,
		MPASResourceReconciledStatus,
	)
}

// ComponentVersionReconciledTotal counts the number times a component version was reconciled.
// [component, version].
var ComponentVersionReconciledTotal = mh.MustRegisterCounterVec(
	"ocm_system",
	metricsComponent,
	"component_version_reconciled_total",
	"Number of times a component version was reconciled",
	"component", "version",
)

// ComponentVersionReconcileFailed counts the number times we failed to reconcile a component version.
// [component].
var ComponentVersionReconcileFailed = mh.MustRegisterCounterVec(
	"ocm_system",
	metricsComponent,
	"component_version_reconcile_failed",
	"Number of times a component version failed to reconcile",
	"component",
)

// ConfigurationReconcileFailed counts the number times we failed to reconcile a Configuration.
// [configuration].
var ConfigurationReconcileFailed = mh.MustRegisterCounterVec(
	"ocm_system",
	metricsComponent,
	"configuration_reconcile_failed",
	"Number of times a configuration failed to reconcile",
	"configuration",
)

// ConfigurationReconcileSuccess counts the number times we succeeded to reconcile a Configuration.
// [configuration].
var ConfigurationReconcileSuccess = mh.MustRegisterCounterVec(
	"ocm_system",
	metricsComponent,
	"configuration_reconcile_success",
	"Number of times a configuration succeeded to reconcile",
	"configuration",
)

// SnapshotReconcileFailed counts the number times we failed to reconcile a Snapshot.
// [configuration].
var SnapshotReconcileFailed = mh.MustRegisterCounterVec(
	"ocm_system",
	metricsComponent,
	"snapshot_reconcile_failed",
	"Number of times a snapshot failed to reconcile",
	"snapshot",
)

// SnapshotReconcileSuccess counts the number times we succeeded to reconcile a Snapshot.
// [configuration].
var SnapshotReconcileSuccess = mh.MustRegisterCounterVec(
	"ocm_system",
	metricsComponent,
	"snapshot_reconcile_success",
	"Number of times a snapshot succeeded to reconcile",
	"snapshot",
)

// LocalizationReconcileFailed counts the number times we failed to reconcile a Localization.
// [localization].
var LocalizationReconcileFailed = mh.MustRegisterCounterVec(
	"ocm_system",
	metricsComponent,
	"localization_reconcile_failed",
	"Number of times a localization failed to reconcile",
	"localization",
)

// LocalizationReconcileSuccess counts the number times we succeeded to reconcile a Localization.
// [localization].
var LocalizationReconcileSuccess = mh.MustRegisterCounterVec(
	"ocm_system",
	metricsComponent,
	"localization_reconcile_success",
	"Number of times a localization succeeded to reconcile",
	"localization",
)

// ResourceReconcileFailed counts the number times we failed to reconcile a resource.
// [resource].
var ResourceReconcileFailed = mh.MustRegisterCounterVec(
	"ocm_system",
	metricsComponent,
	"resource_reconcile_failed",
	"Number of times a resource failed to reconcile",
	"resource",
)

// ResourceReconcileSuccess counts the number times we failed to reconcile a resource.
// [resource].
var ResourceReconcileSuccess = mh.MustRegisterCounterVec(
	"ocm_system",
	metricsComponent,
	"resource_reconcile_success",
	"Number of times a resource succeeded to reconcile",
	"resource",
)

// FluxDeployerReconcileFailed counts the number times we failed to reconcile a flux deployer.
// [deployer].
var FluxDeployerReconcileFailed = mh.MustRegisterCounterVec(
	"ocm_system",
	metricsComponent,
	"flux_deployer_reconcile_failed",
	"Number of times a flux deployer failed to reconcile",
	"deployer",
)

// FluxDeployerReconcileSuccess counts the number times we failed to reconcile a flux deployer.
// [deployer].
var FluxDeployerReconcileSuccess = mh.MustRegisterCounterVec(
	"ocm_system",
	metricsComponent,
	"flux_deployer_reconcile_success",
	"Number of times a flux deployer succeeded to reconcile",
	"deployer",
)

// SnapshotNumberOfBytesReconciled number of bytes reconciled through snapshots.
// [snapshot, digest, component].
var SnapshotNumberOfBytesReconciled = mh.MustRegisterGaugeVec(
	"ocm_system",
	metricsComponent,
	"snapshot_size_bytes",
	"Number of bytes reconciled by a snapshot",
	"snapshot", "digest", "component",
)

// MPASComponentVersionReconciledStatus updates the status of an MPAS component version.
// [product, status].
var MPASComponentVersionReconciledStatus = mh.MustRegisterCounterVec(
	"mpas_system",
	metricsComponent,
	mh.MPASProductComponentVersionCounterLabel,
	"The status of an mpas product.",
	"product", mh.MPASProductInstallationCounterStatusLabel,
)

// MPASLocationReconciledStatus updates the status of an MPAS component version.
// [product, status].
var MPASLocationReconciledStatus = mh.MustRegisterCounterVec(
	"mpas_system",
	metricsComponent,
	mh.MPASProductLocalizationCounterLabel,
	"The status of an mpas product.",
	"product", mh.MPASProductInstallationCounterStatusLabel,
)

// MPASConfigurationReconciledStatus updates the status of an MPAS component version.
// [product, status].
var MPASConfigurationReconciledStatus = mh.MustRegisterCounterVec(
	"mpas_system",
	metricsComponent,
	mh.MPASProductConfigurationCounterLabel,
	"The status of an mpas product.",
	"product", mh.MPASProductInstallationCounterStatusLabel,
)

// MPASDeployerReconciledStatus updates the status of an MPAS component version.
// [product, status].
var MPASDeployerReconciledStatus = mh.MustRegisterCounterVec(
	"mpas_system",
	metricsComponent,
	mh.MPASProductDeployerCounterLabel,
	"The status of an mpas product.",
	"product", mh.MPASProductInstallationCounterStatusLabel,
)

// MPASResourceReconciledStatus updates the status of an MPAS component version.
// [product, status].
var MPASResourceReconciledStatus = mh.MustRegisterCounterVec(
	"mpas_system",
	metricsComponent,
	mh.MPASProductResourceCounterLabel,
	"The status of an mpas product.",
	"product", mh.MPASProductInstallationCounterStatusLabel,
)
