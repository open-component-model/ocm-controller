// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

const (
	// AuthenticatedContextCreationFailedReason is used when the controller failed to create an authenticated context.
	AuthenticatedContextCreationFailedReason = "AuthenticatedContextCreationFailed"

	// CheckVersionFailedReason is used when the controller failed to check for new versions.
	CheckVersionFailedReason = "CheckVersionFailedReason"

	// VerificationFailedReason is used when the signature verification of a component failed.
	VerificationFailedReason = "ComponentVerificationFailed"

	// ComponentVersionInvalidReason is used when the component version is invalid, or we fail to retrieve it.
	ComponentVersionInvalidReason = "ComponentVersionInvalid"

	// ConvertComponentDescriptorFailedReason is used when the Component Descriptor cannot be converted.
	ConvertComponentDescriptorFailedReason = "ConvertComponentDescriptorFailed"

	// NameGenerationFailedReason is used when the component name could not be generated.
	NameGenerationFailedReason = "NameGenerationFailed"

	// CreateOrUpdateComponentDescriptorFailedReason is used when the Component Descriptor cannot be created or updated on the resource.
	CreateOrUpdateComponentDescriptorFailedReason = "CreateOrUpdateComponentDescriptorFailed"

	// ParseReferencesFailedReason is used when the resource references cannot be parsed.
	ParseReferencesFailedReason = "ParseReferencesFailed"

	// ReconcileMutationObjectFailedReason is used when the mutation object cannot be reconciled.
	ReconcileMutationObjectFailedReason = "ReconcileMutationObjectFailed"

	// SourceReasonNotATarArchiveReason is used when the source resource is not a tar archive.
	SourceReasonNotATarArchiveReason = "SourceReasonNotATarArchive"

	// GetResourceFailedReason is used when the resource cannot be retrieved.
	GetResourceFailedReason = "GetResourceFailed"

	// GetComponentDescriptorFailedReason is used when the component descriptor cannot be retrieved.
	GetComponentDescriptorFailedReason = "GetComponentDescriptorFailed"

	// ComponentDescriptorNotFoundReason is used when the component descriptor cannot be found.
	ComponentDescriptorNotFoundReason = "ComponentDescriptorNotFound"

	// ComponentVersionNotFoundReason is used when the component version cannot be found.
	ComponentVersionNotFoundReason = "ComponentVersionNotFound"

	// ComponentVersionNotReadyReason is used when the component version is not ready.
	ComponentVersionNotReadyReason = "ComponentVersionNotReady"

	// CreateOrUpdateSnapshotFailedReason is used when the snapshot cannot be created or updated.
	CreateOrUpdateSnapshotFailedReason = "CreateOrUpdateSnapshotFailed"

	// CreateOrUpdateKustomizationFailedReason is used when the Kustomization cannot be created or updated.
	CreateOrUpdateKustomizationFailedReason = "CreateOrUpdateKustomizationFailed"

	// CreateOrUpdateHelmFailedReason is used when the Kustomization cannot be created or updated.
	CreateOrUpdateHelmFailedReason = "CreateOrUpdateHelmFailed"

	// CreateRepositoryNameReason is used when the generating a new repository name fails.
	CreateRepositoryNameReason = "CreateRepositoryNameFailed"

	// ConfigRefNotReadyWithErrorReason is used when configuration reference is not ready yet with an error.
	ConfigRefNotReadyWithErrorReason = "ConfigRefNotReadyWithError"

	// ConfigRefNotReadyReason is used when configuration ref is not ready yet and there was no error.
	ConfigRefNotReadyReason = "ConfigRefNotReady"

	// SourceRefNotReadyWithErrorReason is used when the source ref is not ready and there was an error.
	SourceRefNotReadyWithErrorReason = "SourceRefNotReadyWithError"

	// SourceRefNotReadyReason is used when the source ref is not ready and there was no error.
	SourceRefNotReadyReason = "SourceRefNotReady"

	// CreatedObjectsNotReadyReason is used when the created resources aren't ready yet.
	CreatedObjectsNotReadyReason = "CreatedObjectsNotReady"

	// PatchStrategicMergeSourceRefNotReadyWithErrorReason is used when source ref for patch strategic merge is not ready and there was an error.
	PatchStrategicMergeSourceRefNotReadyWithErrorReason = "PatchStrategicMergeSourceRefNotReadyWithError"

	// PatchStrategicMergeSourceRefNotReadyReason is used when source ref for patch strategic merge is not ready and there was no error.
	PatchStrategicMergeSourceRefNotReadyReason = "PatchStrategicMergeSourceRefNotReady"

	// SnapshotNameEmptyReason is used for a failure to generate a snapshot name.
	SnapshotNameEmptyReason = "SnapshotNameEmpty"

	// TransferFailedReason is used when we fail to transfer a component.
	TransferFailedReason = "TransferFailed"
)
