// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MutationObject defines any object which produces a snapshot
// +k8s:deepcopy-gen=false
type MutationObject interface {
	client.Object
	SnapshotProducer
	GetSpec() *MutationSpec
	GetStatus() *MutationStatus
}

// MutationSpec defines a common spec between Localization and Configuration.
type MutationSpec struct {
	// +required
	Interval metav1.Duration `json:"interval"`

	// +required
	SourceRef ObjectReference `json:"sourceRef"`

	// +optional
	ConfigRef *ObjectReference `json:"configRef,omitempty"`

	// +optional
	OutputTemplate *SnapshotTemplateSpec `json:"outputTemplate,omitempty"`

	// +optional
	Values *apiextensionsv1.JSON `json:"values,omitempty"`

	// +optional
	PatchStrategicMerge *PatchStrategicMerge `json:"patchStrategicMerge,omitempty"`

	// Suspend stops all operations on this object.
	// +optional
	Suspend bool `json:"suspend,omitempty"`
}

// PatchStrategicMerge contains the source and target details required to perform a strategic merge
type PatchStrategicMerge struct {
	// +required
	Source PatchStrategicMergeSource `json:"source"`

	// +required
	Target PatchStrategicMergeTarget `json:"target"`
}

// PatchStrategicMergeSource contains the details required to retrieve the source from a Flux source
type PatchStrategicMergeSource struct {
	// +required
	SourceRef meta.NamespacedObjectKindReference `json:"sourceRef"`

	// +required
	Path string `json:"path"`
}

// PatchStrategicMergeTarget provides details about the merge target
type PatchStrategicMergeTarget struct {
	Path string `json:"path"`
}

// GetRequeueAfter returns the duration after which the Localization must be
// reconciled again.
func (in MutationSpec) GetRequeueAfter() time.Duration {
	return in.Interval.Duration
}

// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description=""
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message",description=""

// MutationStatus defines a common status for Localizations and Configurations.
type MutationStatus struct {
	// ObservedGeneration is the last reconciled generation.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// +optional
	LatestSnapshotDigest string `json:"latestSnapshotDigest,omitempty"`

	// +optional
	LatestSourceVersion string `json:"latestSourceVersion,omitempty"`

	// +optional
	LatestConfigVersion string `json:"latestConfigVersion,omitempty"`

	// +optional
	LatestPatchSourceVersion string `json:"latestPatchSourceVersio,omitempty"`

	// +optional
	SnapshotName string `json:"snapshotName,omitempty"`
}
