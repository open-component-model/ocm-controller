package v1alpha1

import (
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// MutationSpec defines a common spec between Localization and Configuration.
type MutationSpec struct {
	// +required
	Interval metav1.Duration `json:"interval"`

	// +required
	ComponentVersionRef meta.NamespacedObjectReference `json:"componentVersionRef"`

	// +required
	Source Source `json:"source"`

	// +optional
	ConfigRef *ConfigReference `json:"configRef,omitempty"`

	// +required
	SnapshotTemplate SnapshotTemplateSpec `json:"snapshotTemplate"`

	// +optional
	Values map[string]string `json:"values,omitempty"`

	// +optional
	PatchStrategicMerge *PatchStrategicMerge `json:"patchStrategicMerge,omitempty"`
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
	SourceRef PatchStrategicMergeSourceRef `json:"sourceRef"`

	// +required
	Path string `json:"path"`
}

// PatchStrategicMergeSourceRef contains the flux source identity metadata
type PatchStrategicMergeSourceRef struct {
	// +kubebuilder:validation:Enum=OCIRepository;GitRepository;Bucket
	// +required
	Kind string `json:"kind"`

	// +required
	Name string `json:"name"`

	// +required
	Namespace string `json:"namespace"`
}

// PatchStrategicMergeTarget provides details about the merge target
type PatchStrategicMergeTarget struct {
	Path string `json:"path"`
}

// MutationStatus defines a common status for Localizations and Configurations.
type MutationStatus struct {
	// ObservedGeneration is the last reconciled generation.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +optional
	// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description=""
	// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message",description=""
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// +optional
	LatestSnapshotDigest string `json:"latestSnapshotDigest,omitempty"`

	LatestConfigVersion string `json:"latestConfigVersion,omitempty"`

	// LastAppliedComponentVersion tracks the last applied component version. If there is a change
	// we fire off a reconcile loop to get that new version.
	// +optional
	LastAppliedComponentVersion string `json:"lastAppliedComponentVersion,omitempty"`

	// LastAppliedSourceDigest defines the last seen source digest that has been encountered
	// by this object. Only applicable if Source is a SourceRef.
	// +optional
	LastAppliedSourceDigest string `json:"lastAppliedSourceDigest,omitempty"`

	// LastAppliedConfigSourceDigest defines the last seen config source digest that has been encountered
	// by this object. Only applicable if Source is a SourceRef.
	// +optional
	LastAppliedConfigSourceDigest string `json:"lastAppliedConfigSourceDigest,omitempty"`

	// LastAppliedPatchMergeSourceDigest defines the last seen patch merge source digest that has been encountered
	// by this object. Only applicable if Source is a SourceRef.
	// +optional
	LastAppliedPatchMergeSourceDigest string `json:"lastAppliedPatchMergeSourceDigest,omitempty"`
}

type ConfigReference struct {
	// +required
	Resource Source `json:"resource"`
}

// ResourceRef define a resource.
// TODO: Change this to ocmmetav1.ResourceReference
// The ocmmetav1.ResourceReference can also contain version!
type ResourceRef struct {
	// +required
	Name string `json:"name"`

	// +optional
	Version string `json:"version,omitempty"`

	// +optional
	ExtraIdentity map[string]string `json:"extraIdentity,omitempty"`

	// ReferencePath is a list of references with identities that include this resource.
	//      referencePath:
	//        - name: installer
	// +optional
	ReferencePath []map[string]string `json:"referencePath,omitempty"`
}

// Source defines a possible incoming format for sources that this object needs for further configuration/localization
// steps.
// +kubebuilder:validation:MinProperties=1
type Source struct {
	// +optional
	SourceRef *meta.NamespacedObjectKindReference `json:"sourceRef,omitempty"`
	// +optional
	ResourceRef *ResourceRef `json:"resourceRef,omitempty"`
}

// GetSourceSnapshotKey is a convenient wrapper to get the NamespacedName for a snapshot reference on the object.
func (in MutationSpec) GetSourceSnapshotKey() types.NamespacedName {
	if in.Source.SourceRef == nil {
		return types.NamespacedName{}
	}
	return types.NamespacedName{
		Namespace: in.Source.SourceRef.Namespace,
		Name:      in.Source.SourceRef.Name,
	}
}

// GetRequeueAfter returns the duration after which the Localization must be
// reconciled again.
func (in MutationSpec) GetRequeueAfter() time.Duration {
	return in.Interval.Duration
}
