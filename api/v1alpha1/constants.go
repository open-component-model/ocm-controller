package v1alpha1

const (
	// DefaultRegistryCertificateSecretName is the name of the of certificate secret for client and registry.
	DefaultRegistryCertificateSecretName = "ocm-registry-tls-certs" //nolint:gosec // not a credential
)

// Internal ExtraIdentity keys.
const (
	ComponentNameKey          = "component-name"
	ComponentVersionKey       = "component-version"
	ResourceNameKey           = "resource-name"
	ResourceVersionKey        = "resource-version"
	SourceNameKey             = "source-name"
	SourceNamespaceKey        = "source-namespace"
	SourceArtifactChecksumKey = "source-artifact-checksum"
)

// Externally defined extra identity keys.
const (
	// ResourceHelmChartNameKey if defined, means the resource is a helm resource and the chart should be added
	// to the repository name.
	ResourceHelmChartNameKey = "helmChart"
)

// Log levels.
const (
	// LevelDebug defines the depth at witch debug information is displayed.
	LevelDebug = 4
)
