// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

const (
	// CheckVersionFailedReason is used when the controller failed to check for new versions
	CheckVersionFailedReason = "CheckVersionFailedReason"

	// VerificationFailedReason is used when the signature verification of a component failed.
	VerificationFailedReason = "ComponentVerificationFailed"

	// ComponentVersionInvalidReason is used when the component version is invalid, or we fail to retreive it.
	ComponentVersionInvalidReason = "ComponentVersionInvalid"

	// ConvertComponentDescriptorFailedReason is used when the Component Descriptor cannot be converted.
	ConvertComponentDescriptorFailedReason = "ConvertComponentDescriptorFailed"

	// NameGenerationFailedReason is used when the componentn name could not be generated.
	NameGenerationFailedReason = "NameGenerationFailed"

	// CreateOrUpdateComponentDescriptorFailedReason is used when the Component Descriptor cannot be created or updated on the resource.
	CreateOrUpdateComponentDescriptorFailedReason = "CreateOrUpdateComponentDescriptorFailed"

	// ParseReferencesFailedReason is used when the resource references cannot be parsed.
	ParseReferencesFailedReason = "ParseReferencesFailed"

	// ReconcileMuationObjectFailed is used when the mutation object cannot be reconciled.
	ReconcileMuationObjectFailedReason = "ReconcileMuationObjectFailed"

	// GetResourceFailedReason is used when the resource cannot be retrieved.
	GetResourceFailedReason = "GetResourceFailed"

	// GetComponentDescriptorFailedReason is used when the component descriptor cannot be retrieved.
	GetComponentDescriptorFailedReason = "GetComponentDescriptorFailed"

	// ComponentDescriptorNotFoundReason is used when the component descriptor cannot be found.
	ComponentDescriptorNotFoundReason = "ComponentDescriptorNotFound"

	// CreateOrUpdateSnapshotFailedReason is used when the snapshot cannot be created or updated.
	CreateOrUpdateSnapshotFailedReason = "CreateOrUpdateSnapshotFailed"

	// CreateOrUpdateOCIRepositoryFailedReason is used when the OCIRepository cannot be created or updated.
	CreateOrUpdateOCIRepositoryFailedReason = "CreateOrUpdateOCIRepositoryFailed"

	// CreateRepositoryNameReason is used when the generating a new repository name fails.
	CreateRepositoryNameReason = "CreateRepositoryNameFailed"

	// PatchSnapshotFailedReason is used when the snapshot cannot be patched.
	PatchSnapshotFailedReason = "PatchSnapshotFailed"

	// SnapshotFailedReason is used when the snapshot cannot be created.
	SnapshotFailedReason = "SnapshotFailed"
)
