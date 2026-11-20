package metrics

import (
	"sigs.k8s.io/controller-runtime/pkg/metrics"
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
var ComponentVersionReconciledTotal = MustRegisterCounterVec(
	"ocm_system",
	metricsComponent,
	"component_version_reconciled_total",
	"Number of times a component version was reconciled",
	"component", "version",
)

// ComponentVersionReconcileFailed counts the number times we failed to reconcile a component version.
// [component].
var ComponentVersionReconcileFailed = MustRegisterCounterVec(
	"ocm_system",
	metricsComponent,
	"component_version_reconcile_failed",
	"Number of times a component version failed to reconcile",
	"component",
)

// ConfigurationReconcileFailed counts the number times we failed to reconcile a Configuration.
// [configuration].
var ConfigurationReconcileFailed = MustRegisterCounterVec(
	"ocm_system",
	metricsComponent,
	"configuration_reconcile_failed",
	"Number of times a configuration failed to reconcile",
	"configuration",
)

// ConfigurationReconcileSuccess counts the number times we succeeded to reconcile a Configuration.
// [configuration].
var ConfigurationReconcileSuccess = MustRegisterCounterVec(
	"ocm_system",
	metricsComponent,
	"configuration_reconcile_success",
	"Number of times a configuration succeeded to reconcile",
	"configuration",
)

// SnapshotReconcileFailed counts the number times we failed to reconcile a Snapshot.
// [configuration].
var SnapshotReconcileFailed = MustRegisterCounterVec(
	"ocm_system",
	metricsComponent,
	"snapshot_reconcile_failed",
	"Number of times a snapshot failed to reconcile",
	"snapshot",
)

// SnapshotReconcileSuccess counts the number times we succeeded to reconcile a Snapshot.
// [configuration].
var SnapshotReconcileSuccess = MustRegisterCounterVec(
	"ocm_system",
	metricsComponent,
	"snapshot_reconcile_success",
	"Number of times a snapshot succeeded to reconcile",
	"snapshot",
)

// LocalizationReconcileFailed counts the number times we failed to reconcile a Localization.
// [localization].
var LocalizationReconcileFailed = MustRegisterCounterVec(
	"ocm_system",
	metricsComponent,
	"localization_reconcile_failed",
	"Number of times a localization failed to reconcile",
	"localization",
)

// LocalizationReconcileSuccess counts the number times we succeeded to reconcile a Localization.
// [localization].
var LocalizationReconcileSuccess = MustRegisterCounterVec(
	"ocm_system",
	metricsComponent,
	"localization_reconcile_success",
	"Number of times a localization succeeded to reconcile",
	"localization",
)

// ResourceReconcileFailed counts the number times we failed to reconcile a resource.
// [resource].
var ResourceReconcileFailed = MustRegisterCounterVec(
	"ocm_system",
	metricsComponent,
	"resource_reconcile_failed",
	"Number of times a resource failed to reconcile",
	"resource",
)

// ResourceReconcileSuccess counts the number times we failed to reconcile a resource.
// [resource].
var ResourceReconcileSuccess = MustRegisterCounterVec(
	"ocm_system",
	metricsComponent,
	"resource_reconcile_success",
	"Number of times a resource succeeded to reconcile",
	"resource",
)

// FluxDeployerReconcileFailed counts the number times we failed to reconcile a flux deployer.
// [deployer].
var FluxDeployerReconcileFailed = MustRegisterCounterVec(
	"ocm_system",
	metricsComponent,
	"flux_deployer_reconcile_failed",
	"Number of times a flux deployer failed to reconcile",
	"deployer",
)

// FluxDeployerReconcileSuccess counts the number times we failed to reconcile a flux deployer.
// [deployer].
var FluxDeployerReconcileSuccess = MustRegisterCounterVec(
	"ocm_system",
	metricsComponent,
	"flux_deployer_reconcile_success",
	"Number of times a flux deployer succeeded to reconcile",
	"deployer",
)

// SnapshotNumberOfBytesReconciled number of bytes reconciled through snapshots.
// [snapshot, digest, component].
var SnapshotNumberOfBytesReconciled = MustRegisterGaugeVec(
	"ocm_system",
	metricsComponent,
	"snapshot_size_bytes",
	"Number of bytes reconciled by a snapshot",
	"snapshot", "digest", "component",
)

// MPASComponentVersionReconciledStatus updates the status of an MPAS component version.
// [product, status].
var MPASComponentVersionReconciledStatus = MustRegisterCounterVec(
	"mpas_system",
	metricsComponent,
	MPASProductComponentVersionCounterLabel,
	"The status of an mpas product.",
	"product", MPASProductInstallationCounterStatusLabel,
)

// MPASLocationReconciledStatus updates the status of an MPAS component version.
// [product, status].
var MPASLocationReconciledStatus = MustRegisterCounterVec(
	"mpas_system",
	metricsComponent,
	MPASProductLocalizationCounterLabel,
	"The status of an mpas product.",
	"product", MPASProductInstallationCounterStatusLabel,
)

// MPASConfigurationReconciledStatus updates the status of an MPAS component version.
// [product, status].
var MPASConfigurationReconciledStatus = MustRegisterCounterVec(
	"mpas_system",
	metricsComponent,
	MPASProductConfigurationCounterLabel,
	"The status of an mpas product.",
	"product", MPASProductInstallationCounterStatusLabel,
)

// MPASDeployerReconciledStatus updates the status of an MPAS component version.
// [product, status].
var MPASDeployerReconciledStatus = MustRegisterCounterVec(
	"mpas_system",
	metricsComponent,
	MPASProductDeployerCounterLabel,
	"The status of an mpas product.",
	"product", MPASProductInstallationCounterStatusLabel,
)

// MPASResourceReconciledStatus updates the status of an MPAS component version.
// [product, status].
var MPASResourceReconciledStatus = MustRegisterCounterVec(
	"mpas_system",
	metricsComponent,
	MPASProductResourceCounterLabel,
	"The status of an mpas product.",
	"product", MPASProductInstallationCounterStatusLabel,
)
