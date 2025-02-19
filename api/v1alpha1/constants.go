package v1alpha1

const (
	// DefaultRegistryCertificateSecretName is the name of the of certificate secret for client and registry.
	DefaultRegistryCertificateSecretName = "ocm-registry-tls-certs" // nolint:gosec // not a credential
)

// Internal ExtraIdentity keys.
const (
	ComponentNameKey          = "component-name"
	ComponentVersionKey       = "component-version"
	ResourceNameKey           = "resource-name"
	ResourceVersionKey        = "resource-version"
	ResourceRefPath           = "resource-reference-path"
	SourceNameKey             = "source-name"
	SourceNamespaceKey        = "source-namespace"
	SourceArtifactChecksumKey = "source-artifact-checksum"
	MutationObjectUUIDKey     = "mutation-object-uuid"
)

// Externally defined extra identity keys.
const (
	// ResourceHelmChartVersion is needed information in case of configuration and localization objects
	// for helm charts, because it cannot be determined from the existing resource and the generated
	// layer tag _needs_ to match with the chart's version for Flux to correctly deploy it using OCIRegistry.
	ResourceHelmChartVersion = "chartVersion"
)

// Log levels.
const (
	// LevelDebug defines the depth at witch debug information is displayed.
	LevelDebug = 4
)

// Ocm credential config key for secrets.
const (
	// OCMCredentialConfigKey defines the secret key to look for in case a user provides an ocm credential config.
	OCMCredentialConfigKey = ".ocmcredentialconfig" // nolint:gosec // it isn't a cred
)
