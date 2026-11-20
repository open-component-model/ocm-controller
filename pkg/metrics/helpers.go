package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

const (
	// MPASStatusSuccess signifies that an MPAS installation has been finished successfully.
	MPASStatusSuccess = "success"
)

const (
	// MPASProductInstallationCounterStatusLabel tracks mpas product installation counts and status.
	MPASProductInstallationCounterStatusLabel = "mpas_product_status"
)

const (
	// MPASProductComponentVersionCounterLabel tracks the status of a component version belonging to a product.
	MPASProductComponentVersionCounterLabel = "mpas_component_version_installed"
	// MPASProductLocalizationCounterLabel tracks the status of a localization belonging to a product.
	MPASProductLocalizationCounterLabel = "mpas_localization_installed"
	// MPASProductConfigurationCounterLabel tracks the status of a configuration belonging to a product.
	MPASProductConfigurationCounterLabel = "mpas_configuration_installed"
	// MPASProductDeployerCounterLabel tracks the status of a deployer belonging to a product.
	MPASProductDeployerCounterLabel = "mpas_deployer_installed"
	// MPASProductResourceCounterLabel tracks the status of a resource belonging to a product.
	MPASProductResourceCounterLabel = "mpas_resource_installed"
)

// MustRegisterCounterVec creates and registers a counter vector.
// Must be called from `init`.
func MustRegisterCounterVec(namespace, component, name, help string, labelNames ...string) *prometheus.CounterVec {
	m := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: component,
		Name:      name,
		Help:      help,
	}, labelNames)
	prometheus.MustRegister(m)

	return m
}

// MustRegisterGaugeVec creates and registers a gauge vector.
// Must be called from `init`.
func MustRegisterGaugeVec(namespace, component, name, help string, labelNames ...string) *prometheus.GaugeVec {
	m := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: component,
		Name:      name,
		Help:      help,
	}, labelNames)
	prometheus.MustRegister(m)

	return m
}
