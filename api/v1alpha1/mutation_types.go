package v1alpha1

import (
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MutationObject defines any object which produces a snapshot
// +k8s:deepcopy-gen=false
type MutationObject interface {
	SnapshotWriter
	GetSpec() *MutationSpec
	GetStatus() *MutationStatus
}

// MutationSpec defines a common spec for Localization and Configuration of OCM resources.
type MutationSpec struct {
	// +required
	Interval metav1.Duration `json:"interval,omitempty"`

	// +required
	SourceRef ObjectReference `json:"sourceRef,omitempty"`

	// +optional
	ConfigRef *ObjectReference `json:"configRef,omitempty"`

	// +optional
	Values *apiextensionsv1.JSON `json:"values,omitempty"`

	// +optional
	ValuesFrom *ValuesSource `json:"valuesFrom,omitempty"`

	// +optional
	PatchStrategicMerge *PatchStrategicMerge `json:"patchStrategicMerge,omitempty"`

	// Suspend stops all operations on this object.
	// +optional
	Suspend bool `json:"suspend,omitempty"`
}

// ValuesSource provides access to values from an external Source such as a ConfigMap or GitRepository or ObjectReference.
// An optional subpath defines the path within the source from which the values should be resolved.
type ValuesSource struct {
	// +optional
	FluxSource *FluxValuesSource `json:"fluxSource,omitempty"`
	// +optional
	ConfigMapSource *ConfigMapSource `json:"configMapSource,omitempty"`
	// +optional
	SourceRef *ObjectReference `json:"sourceRef,omitempty"`
}

type ConfigMapSource struct {
	// +required
	SourceRef meta.LocalObjectReference `json:"sourceRef"`
	// +required
	Key string `json:"key"`
	// +optional
	SubPath string `json:"subPath,omitempty"`
	// Optional marks this ConfigMapSource as optional. When set, a not found
	// error for the configmap reference is ignored, but any Key, Subpath or
	// transient error will still result in a reconciliation failure.
	// +optional
	Optional bool `json:"optional,omitempty"`
}

type FluxValuesSource struct {
	// +required
	SourceRef meta.NamespacedObjectKindReference `json:"sourceRef"`

	// +required
	Path string `json:"path"`

	// +optional
	SubPath string `json:"subPath,omitempty"`
}

// PatchStrategicMerge contains the source and target details required to perform a strategic merge.
type PatchStrategicMerge struct {
	// +required
	Source PatchStrategicMergeSource `json:"source"`

	// +required
	Target PatchStrategicMergeTarget `json:"target"`
}

// PatchStrategicMergeSource contains the details required to retrieve the source from a Flux source.
type PatchStrategicMergeSource struct {
	// +required
	SourceRef meta.NamespacedObjectKindReference `json:"sourceRef"`

	// +required
	Path string `json:"path"`
}

// PatchStrategicMergeTarget provides details about the merge target.
type PatchStrategicMergeTarget struct {
	Path string `json:"path"`
}

// GetRequeueAfter returns the duration after which the Localization must be
// reconciled again.
func (in MutationSpec) GetRequeueAfter() time.Duration {
	return in.Interval.Duration
}

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
